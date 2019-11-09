package org.sgdan.podreaper

import com.fasterxml.jackson.databind.JavaType
import com.fasterxml.jackson.databind.ObjectMapper
import io.fabric8.kubernetes.api.model.LimitRangeItem
import io.fabric8.kubernetes.api.model.LimitRangeItemBuilder
import io.fabric8.kubernetes.api.model.Quantity
import io.fabric8.kubernetes.api.model.ResourceQuota
import io.fabric8.kubernetes.client.KubernetesClient
import mu.KotlinLogging
import java.lang.Long.max
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter

private val log = KotlinLogging.logger {}
private val om = ObjectMapper()
private val jType: JavaType = om.typeFactory.constructParametricType(
        Map::class.java, String::class.java, NamespaceConfig::class.java)
private val formatter: DateTimeFormatter = DateTimeFormatter.ofPattern("HH:mm z")

/**
 * A read-only snapshot of the relevant data from the kubernetes
 * cluster describing the namespaces we're interested in.
 */
data class Status(val zone: ZoneId, // must be provided (set from config)
                  val ignoredNamespaces: Set<String>,
                  val zdt: ZonedDateTime = ZonedDateTime.now(zone),
                  val now: Long = zdt.toInstant().toEpochMilli(),
                  val clock: String = zdt.format(formatter),
                  val settings: Map<String, NamespaceConfig> = emptyMap(),
                  val namespaces: List<NamespaceStatus> = emptyList(),
                  val error: String = "")

data class NamespaceStatus(val name: String,
                           val hasLimitRange: Boolean,
                           val hasResourceQuota: Boolean,
                           val hasDownQuota: Boolean, // the resource quote with zero limit used to disable namespace
                           val canExtend: Boolean,
                           val memUsed: Int,
                           val memLimit: Int,
                           val autoStartHour: Int?,
                           val remaining: String,
                           val lastScheduled: ZonedDateTime?,
                           val lastStarted: Long)

/** Stored in config map in k8s cluster for each namespace */
data class NamespaceConfig(
        val autoStartHour: Int? = null,
        val lastStarted: Long = 0)

/**
 * @return a read only status snapshot by reading the data from the k8s API
 *
 * @param status will use the settings already in the given status
 */
fun read(client: KubernetesClient, status: Status): Status = try {
    readNamespaces(client, readSettings(client, status))
} catch (e: Exception) {
    log.error(e) { "Unable to read status" }
    status.copy(error = "Unable to read status: $e")
}

fun parseConfig(json: String?): Map<String, NamespaceConfig> = try {
    om.readValue<Map<String, NamespaceConfig>>(json, jType)
} catch (e: NullPointerException) {
    log.warn { "Unable to parse config: null" }
    emptyMap()
} catch (e: Exception) {
    log.warn(e) { "Unable to parse config, ignoring: $json" }
    emptyMap()
}

/**
 * @return the namespace settings stored in the config map
 */
fun readSettings(client: KubernetesClient, status: Status): Status {
    val cm = client.configMaps().inNamespace(REAPER_NAMESPACE).withName(CONFIG_MAP_NAME).get()
    return status.copy(settings = parseConfig(cm?.data?.get(CONFIG)))
}

fun readNamespaces(client: KubernetesClient, status: Status): Status {
    val namespaces = client.namespaces().list().items.map { it.metadata.name }
            .filter { !status.ignoredNamespaces.contains(it) }
            .map { readNamespace(client, it, status) }
    return status.copy(namespaces = namespaces)
}

/**
 * @return the number of seconds remaining for this namespace
 */
fun remainingSeconds(lastStarted: Long, now: Long) =
        max(lastStarted + WINDOW * 60 * 60 * 1000 - now, 0) / 1000


fun remainingTime(remaining: Long): String {
    val m = remaining / 60
    val h = (m / 60) % WINDOW

    return when {
        m <= 0 || m >= WINDOW * 60 -> ""
        h > 0 -> "${h}h %02dm".format(m % 60)
        else -> "${m % 60}m"
    }
}

fun readNamespace(client: KubernetesClient, name: String, status: Status): NamespaceStatus {
    val rq = readResourceQuota(client, name)
    val lastStarted = status.settings[name]?.lastStarted ?: 0
    val remaining = remainingSeconds(lastStarted, status.now)
    val autoStartHour = status.settings[name]?.autoStartHour
    return NamespaceStatus(
            name,
            hasLimitRange(client, name),
            rq != null,
            hasDownQuota(client, name),
            remaining < (WINDOW - 1) * 60 * 60,
            toGigs(rq?.status?.used?.get(LIMITS_MEMORY)?.amount),
            toGigs(rq?.spec?.hard?.get(LIMITS_MEMORY)?.amount),
            autoStartHour,
            remainingTime(remaining),
            lastScheduled(autoStartHour, status.zdt),
            lastStarted
    )
}

fun hasDownQuota(client: KubernetesClient, namespace: String): Boolean =
        client.resourceQuotas().inNamespace(namespace)
                .withName(DOWN_QUOTA_NAME).get() != null

fun readResourceQuota(client: KubernetesClient, namespace: String): ResourceQuota? {
    return client.resourceQuotas().inNamespace(namespace).withName(RESOURCE_QUOTA_NAME).get()
}

fun defaultLimitRangeItem(): LimitRangeItem = LimitRangeItemBuilder()
        .withDefault(mapOf(MEMORY to Quantity(POD_LIMIT)))
        .withDefaultRequest(mapOf(MEMORY to Quantity(POD_REQUEST)))
        .build()!!

fun hasLimitRange(client: KubernetesClient, namespace: String): Boolean =
        client.limitRanges().inNamespace(namespace)
                .withName(LIMIT_RANGE_NAME)
                .get()?.let {
                    it.spec?.limits?.contains(defaultLimitRangeItem())
                } ?: false

operator fun Regex.contains(text: CharSequence): Boolean = matches(text)
private fun toGigs(value: String?) = when (value) {
    null -> 0
    in Regex("[0-9]+Gi") -> value.removeSuffix("Gi").toInt()
    in Regex("[0-9]+Mi") -> value.removeSuffix("Mi").toInt() / 1000
    else -> 0
}
