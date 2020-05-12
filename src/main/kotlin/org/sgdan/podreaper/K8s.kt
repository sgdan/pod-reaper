package org.sgdan.podreaper

import com.fasterxml.jackson.databind.JavaType
import com.fasterxml.jackson.databind.ObjectMapper
import io.fabric8.kubernetes.api.model.*
import io.fabric8.kubernetes.client.KubernetesClient
import io.fabric8.kubernetes.client.dsl.Resource
import mu.KotlinLogging
import java.math.BigDecimal
import java.time.ZoneId
import java.time.ZonedDateTime
import kotlin.math.max

const val REAPER_NAMESPACE = "podreaper"
const val CONFIG_MAP_NAME = "podreaper-config"
const val CONFIG = "config"
const val LIMIT_RANGE_NAME = "reaper-limit"
const val MEMORY = "memory"
const val CONTAINER = "Container"
const val RESOURCE_QUOTA_NAME = "reaper-quota"
const val DOWN_QUOTA_NAME = "reaper-down-quota"
const val LIMITS_MEMORY = "limits.memory"
const val WINDOW = 8 // Eight hour up-time window

val POD_REQUEST = Quantity("512Mi")
val POD_LIMIT = Quantity("512Mi")
val DEFAULT_QUOTA = Quantity("10Gi")

private val log = KotlinLogging.logger {}
private val om = ObjectMapper()
private val jType: JavaType = om.typeFactory.constructParametricType(
        Map::class.java, String::class.java, NamespaceConfig::class.java)

/**
 * Wraps read and write operations on the kubernetes cluster
 */
class K8s(private val client: KubernetesClient,
          private val ignoredNamespaces: Set<String>,
          private val zoneId: ZoneId) {

    // getter methods below just read from the cluster

    /**
     * @return the namespace settings stored in the config map
     */
    fun getSettings(): Map<String, NamespaceConfig> = try {
        val cm = client.configMaps()
                .inNamespace(REAPER_NAMESPACE)
                .withName(CONFIG_MAP_NAME)
                .get()
        parseConfig(cm?.data?.get(CONFIG))
    } catch (e: Exception) {
        log.error { "Unable to parse config: ${e.message}" }
        emptyMap()
    }

    /**
     * Parse the config map containing last started times and auto start
     * hours for namespaces
     */
    private fun parseConfig(json: String?): Map<String, NamespaceConfig> =
            om.readValue<Map<String, NamespaceConfig>>(json, jType)

    private val limitRange = LimitRangeBuilder()
            .withNewMetadata().withName(LIMIT_RANGE_NAME).endMetadata()
            .withNewSpec().addNewLimit()
            .withDefault(mapOf(MEMORY to POD_LIMIT))
            .withDefaultRequest(mapOf(MEMORY to POD_REQUEST))
            .withType(CONTAINER)
            .endLimit().endSpec().build()

    fun getLimitRange(namespace: String)
            : Resource<LimitRange, DoneableLimitRange> =
            client.limitRanges().inNamespace(namespace)
                    .withName(LIMIT_RANGE_NAME)

    fun getHasLimitRange(namespace: String): Boolean = try {
        getLimitRange(namespace)
                .get()?.spec?.limits?.get(0)?.let {
                    POD_LIMIT == it.default?.get(MEMORY)
                            && POD_REQUEST == it.defaultRequest?.get(MEMORY)
        } ?: false
    } catch (e: Exception) {
        log.error { "Unable to get limit range for $namespace: ${e.message}" }
        false
    }

    fun loadNamespace(name: String, autoStartHour: Int?, prevStarted: Long): NamespaceStatus {
        val zdt = ZonedDateTime.now(zoneId)
        val now = zdt.toEpochSecond() * 1000
        val rq = getResourceQuota(name) // access k8s
        val lastScheduled = lastScheduled(autoStartHour, zdt)
        val lastScheduledMillis = lastScheduled.toEpochSecond() * 1000
        val lastStarted = max(prevStarted, lastScheduledMillis)
        val remaining = remainingSeconds(lastStarted, now)
        return NamespaceStatus(
                // UI frontend
                name = name,
                hasDownQuota = getHasDownQuota(name), // access k8s
                canExtend = remaining < (WINDOW - 1) * 60 * 60,
                memUsed = toGigs(rq?.status?.used?.get(LIMITS_MEMORY)),
                memLimit = toGigs(rq?.spec?.hard?.get(LIMITS_MEMORY)),
                autoStartHour = autoStartHour,
                remaining = remainingTime(remaining),

                // backend only
                hasResourceQuota = rq != null,
                lastScheduled = lastScheduled,
                lastStarted = lastStarted
        )
    }

    fun getResourceQuota(namespace: String): ResourceQuota? {
        return client.resourceQuotas().inNamespace(namespace).withName(RESOURCE_QUOTA_NAME).get()
    }

    fun getHasDownQuota(namespace: String): Boolean =
            client.resourceQuotas().inNamespace(namespace)
                    .withName(DOWN_QUOTA_NAME).get() != null

    fun getLiveNamespaces(): Set<String> = client.namespaces().list().items
            .map { it.metadata.name }
            .filter { !ignoredNamespaces.contains(it) }
            .toSet()

    fun getExists(namespace: String) =
            client.namespaces().withName(namespace).get() != null

    // setter methods below alter the cluster state

    /**
     * Remove any "reaper-down" resource quota so the namespace
     * will be running
     */
    fun bringUp(namespace: String) {
        client.resourceQuotas()
                .inNamespace(namespace)
                .withName(DOWN_QUOTA_NAME)
                .delete()
        log.info { "Bringing up $namespace" }
    }

    fun bringDown(namespace: String) {
        if (!getHasDownQuota(namespace)) {
            setResourceQuota(namespace, DOWN_QUOTA_NAME, Quantity("0"))
            log.info { "Bringing down $namespace" }
        }
    }

    fun setResourceQuota(ns: String, name: String, limit: Quantity) {
        try {
            client.resourceQuotas().inNamespace(ns).createOrReplaceWithNew()
                    .withNewMetadata().withName(name).withNamespace(ns).endMetadata()
                    .withNewSpec().withHard(mapOf(LIMITS_MEMORY to limit))
                    .endSpec().done()
        } catch (e: Exception) {
            log.error { "Unable to create resource quota for $ns: ${e.message}" }
        }
    }

    fun setLimit(ns: String, limit: Int) {
        client.resourceQuotas().inNamespace(ns).withName(RESOURCE_QUOTA_NAME)
                .edit().editSpec()
                .addToHard(LIMITS_MEMORY, Quantity("${limit}Gi"))
                .endSpec().done()
    }

    fun saveSettings(settings: Map<String, NamespaceConfig>) {
        try {
            val json = om.writeValueAsString(settings)
            val cm = ConfigMapBuilder()
                    .withNewMetadata().withName(CONFIG_MAP_NAME).endMetadata()
                    .addToData(CONFIG, json).build()
            client.configMaps()
                    .inNamespace((REAPER_NAMESPACE))
                    .withName(CONFIG_MAP_NAME)
                    .createOrReplace(cm)
        } catch (e: Exception) {
            log.error { "Unable to save settings: ${e.message}" }
        }
    }

    fun setLimitRange(namespace: String) = try {
        getLimitRange(namespace).createOrReplace(limitRange)
        log.info { "Created limit range for $namespace" }
    } catch (e: Exception) {
        log.error { "Unable to create limit range for $namespace: ${e.message}" }
    }

    fun deletePods(name: String) {
        client.pods().inNamespace(name).let {
            val n = it.list().items.size
            if (n > 0 && it.delete()) log.info { "Deleted $n pods in $name" }
        }
    }
}

fun toGigs(value: Quantity?) = when {
    value == null -> 0
    value.format == "Gi" -> value.amount.toInt()
    value.format == "Mi" -> value.amount.toInt() / 1000
    else -> Quantity.getAmountInBytes(value).divide(BigDecimal(1000 * 1000)).toInt()
}
