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
                           val lastStarted: Long,
                           val lastRefreshed: Long = 0)

/** Stored in config map in k8s cluster for each namespace */
data class NamespaceConfig(
        val autoStartHour: Int? = null,
        val lastStarted: Long = 0)

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
fun readSettings(client: KubernetesClient): Map<String, NamespaceConfig> {
    val cm = client.configMaps().inNamespace(REAPER_NAMESPACE).withName(CONFIG_MAP_NAME).get()
    return parseConfig(cm?.data?.get(CONFIG))
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
fun toGigs(value: String?) = when (value) {
    null -> 0
    in Regex("[0-9]+Gi") -> value.removeSuffix("Gi").toInt()
    in Regex("[0-9]+Mi") -> value.removeSuffix("Mi").toInt() / 1000
    else -> 0
}
