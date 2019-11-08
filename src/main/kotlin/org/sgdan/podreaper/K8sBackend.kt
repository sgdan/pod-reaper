package org.sgdan.podreaper

import com.fasterxml.jackson.databind.ObjectMapper
import io.fabric8.kubernetes.api.model.ConfigMapBuilder
import io.fabric8.kubernetes.api.model.LimitRangeBuilder
import io.fabric8.kubernetes.api.model.Quantity
import io.fabric8.kubernetes.api.model.ResourceQuota
import io.fabric8.kubernetes.client.DefaultKubernetesClient
import io.fabric8.kubernetes.client.KubernetesClient
import io.micronaut.context.annotation.Factory
import io.micronaut.context.annotation.Requires
import io.micronaut.context.annotation.Value
import io.micronaut.scheduling.annotation.Scheduled
import mu.KotlinLogging
import java.lang.System.currentTimeMillis
import java.time.DayOfWeek
import java.time.Instant
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.temporal.ChronoUnit
import javax.inject.Singleton

const val LIMITS_MEMORY = "limits.memory"

private val log = KotlinLogging.logger {}
private val om = ObjectMapper()

class ReaperException(msg: String, t: Throwable? = null) : Exception(msg, t)

data class Setting(
        val startHour: Int? = null,
        val lastStarted: Long = 0)

data class MemStats(
        val memLimit: Int = 0,
        val memUsed: Int = 0)

operator fun Regex.contains(text: CharSequence): Boolean = matches(text)
private fun toGigs(value: String?) = when (value) {
    null -> 0
    in Regex("[0-9]+Gi") -> value.removeSuffix("Gi").toInt()
    in Regex("[0-9]+Mi") -> value.removeSuffix("Mi").toInt() / 1000
    else -> 0
}

private fun isWeekend(day: DayOfWeek) =
        setOf(DayOfWeek.SATURDAY, DayOfWeek.SUNDAY).contains(day)

/**
 * @return same time on most recent weekday
 */
private fun weekday(now: ZonedDateTime): ZonedDateTime =
        if (isWeekend(now.dayOfWeek)) weekday(now.minusDays(1)) else now

private fun hoursFrom(earlier: ZonedDateTime, later: ZonedDateTime) =
        ChronoUnit.HOURS.between(earlier, later)

private fun mostRecent(a: ZonedDateTime, b: ZonedDateTime) = if (a.isAfter(b)) a else b

@Factory
class ClientFactory {
    @Singleton
    fun client() = DefaultKubernetesClient()
}

@Singleton
@Requires(property = "backend.kubernetes.enabled", value = "true")
class K8sBackend(val client: KubernetesClient) : Backend {
    @Value("\${backend.kubernetes.ignore}")
    lateinit var ignore: Set<String>

    @Value("\${backend.kubernetes.zoneid}")
    lateinit var zoneid: String

    private fun zone(): ZoneId = ZoneId.of(zoneid)

    private var settings = loadSettings()

    /**
     * @return same time as "now" but from most recent weekday, or null
     *         if no start hour has been specified
     */
    fun lastScheduled(startHour: Int?, now: ZonedDateTime) = startHour?.let {
        val start = weekday(now.withHour(it))
        if (now.isAfter(start)) start else weekday(start.minusDays(1))
    } ?: toZDT(0)

    private fun toZDT(millis: Long): ZonedDateTime =
            ZonedDateTime.ofInstant(Instant.ofEpochMilli(millis), zone())

    /**
     * @return list of names of non-system namespaces
     */
    fun getNamespaces(): List<String> =
            client.namespaces().list().items
                    .map { it.metadata.name }
                    .filter { !ignore.contains(it) }

    /**
     * @return true if the given namespace is "up", i.e. there is
     *              no resource quota called "reaper-down"
     */
    fun isUp(namespace: String): Boolean {
        val down = client.resourceQuotas().inNamespace(namespace)
                .withName("reaper-down").get()

        // directly after the resource quota has been deleted it may
        // still exist but will have a non-null deletion timestamp
        return down?.metadata?.deletionTimestamp != null || down == null
    }

    /**
     * Remove any "reaper-down" resource quota so the namespace
     * will be running
     */
    fun bringUp(namespace: String) {
        client.resourceQuotas().inNamespace(namespace)
                .withName("reaper-down").delete()
    }

    /**
     * Bring down a namespace by creating a "reaper-down" resource quota with
     * memory limit zero.
     */
    fun bringDown(namespace: String) {
        "reaper-down".let {
            if (getResourceQuota(namespace, it) == null)
                createResourceQuota(namespace, it, 0)
            else setLimit(namespace, it, 0)
        }
    }

    fun setLimit(namespace: String, name: String, limit: Int) {
        try {
            client.resourceQuotas().inNamespace(namespace).withName(name).edit()
                    .editSpec()
                    .addToHard(LIMITS_MEMORY, Quantity("${limit}Gi"))
                    .endSpec().done()
        } catch (e: Exception) {
            throw ReaperException("Unable to set limit for $namespace/$name/$limit", e)
        }
    }

    fun getLimitUsed(namespace: String, name: String): Pair<Int, Int> {
        return client.resourceQuotas().inNamespace(namespace).withName(name).get()?.let {
            Pair(
                    toGigs(it.spec?.hard?.get(LIMITS_MEMORY)?.amount),
                    toGigs(it.status?.used?.get(LIMITS_MEMORY)?.amount)
            )
        } ?: Pair(0, 0)
    }

    fun createResourceQuota(namespace: String, name: String, limit: Int): ResourceQuota? = try {
        client.resourceQuotas().inNamespace(namespace).createNew()
                .withNewMetadata().withName(name).withNamespace(namespace).endMetadata()
                .withNewSpec().withHard(mapOf(LIMITS_MEMORY to Quantity("${limit}Gi")))
                .endSpec().done()
    } catch (e: Exception) {
        log.error(e) { "Unable to create resource quota $namespace/$name/$limit" }
        null
    }

    private fun getResourceQuota(namespace: String, name: String): ResourceQuota? =
            client.resourceQuotas().inNamespace(namespace).withName(name).get()

    private fun saveSettings(settings: Map<String, Setting>) {
        val json = om.writeValueAsString(settings)
        val cm = ConfigMapBuilder()
                .withNewMetadata().withName("podreaper-config").endMetadata()
                .addToData("config", json).build()
        client.configMaps().inNamespace(("podreaper")).withName("podreaper-config").createOrReplace(cm)
    }

    private fun loadSettings(): Map<String, Setting> {
        val cm = client.configMaps().inNamespace("podreaper")
                .withName("podreaper-config").get()
        val json: String? = cm?.data?.get("config")
        val jType = om.typeFactory.constructParametricType(
                Map::class.java, String::class.java, Setting::class.java)
        return json?.let { om.readValue<Map<String, Setting>>(it, jType) }
                ?: emptyMap<String, Setting>()
    }

    private fun reap(ns: String, lastStartedMillis: Long, startHour: Int?) {
        val up = isUp(ns)
        val now = ZonedDateTime.now(zone())
        val lastScheduled = lastScheduled(startHour, now)
        val lastStarted = mostRecent(lastScheduled, toZDT(lastStartedMillis))
        val shouldRun = hoursFrom(lastStarted, now) < 8
        log.debug { "reap: $ns/$now/$lastScheduled/$lastStarted/$shouldRun" }

        // change up/down state
        if (up && !shouldRun) bringDown(ns)
        if (!up && shouldRun) bringUp(ns)

        // kill any pods that are running
        if (!shouldRun) client.pods().inNamespace(ns).delete()

        // update settings
        settings = settings.plus(ns to Setting(startHour, lastStarted.toEpochSecond() * 1000))
    }

    private fun initLimitRange(namespace: String) = try {
        val lrb = LimitRangeBuilder()
                .withNewMetadata().withName("reaperlimit").endMetadata()
                .withNewSpec().addNewLimit()
                .withDefault(mapOf("memory" to Quantity("512Mi")))
                .withDefaultRequest(mapOf("memory" to Quantity("512Mi")))
                .withType("Container")
                .endLimit().endSpec().build()
        client.limitRanges().inNamespace(namespace).withName("reaperlimit").createOrReplace(lrb)
    } catch (e: Exception) {
        log.error(e) { "Unable to create limit range in $namespace" }
        null
    }

    /**
     * Check that the given namespace has limit range and resource quota defined,
     * then return the current memory stats.
     */
    private fun check(namespace: String): MemStats {
        client.limitRanges().inNamespace(namespace).withName("reaperlimit").get()
                ?: initLimitRange(namespace)
        val rq = getResourceQuota(namespace, "reaper-up")
                ?: createResourceQuota(namespace, "reaper-up", 10)
        return MemStats(toGigs(rq?.spec?.hard?.get(LIMITS_MEMORY)?.amount),
                toGigs(rq?.status?.used?.get(LIMITS_MEMORY)?.amount))
    }

    @Synchronized
    @Scheduled(fixedDelay = "30s", initialDelay = "5s")
    fun reap() {
        getNamespaces().forEach {
            reap(it, settings[it]?.lastStarted ?: 0L, settings[it]?.startHour)
        }
        saveSettings(settings)
    }

    @Synchronized
    override fun getStatus(): Status {
        return Status(getNamespaces().map { ns ->
            val stats = check(ns)
            NamespaceStatus(
                    ns,
                    isUp(ns),
                    stats.memUsed,
                    stats.memLimit,
                    settings[ns]?.startHour,
                    (settings[ns]?.lastStarted ?: 0L) + (8 * 60 * 60 * 1000)
            )
        }, ZonedDateTime.now(zone()).format(formatter))
    }

    @Synchronized
    override fun setMemLimit(namespace: String, limit: Int): Status {
        setLimit(namespace, "reaper-up", limit)
        return getStatus()
    }

    @Synchronized
    override fun setStartHour(namespace: String, startHour: Int?): Status {
        val namespaces = getNamespaces()
        settings = settings.plus(namespace to
                Setting(startHour, settings[namespace]?.lastStarted ?: 0L))
                .filterKeys { namespaces.contains(it) }
        saveSettings(settings)
        return getStatus()
    }

    @Synchronized
    override fun extend(namespace: String): Status {
        val namespaces = getNamespaces()
        settings = settings.plus(namespace to
                Setting(settings[namespace]?.startHour, currentTimeMillis()))
                .filterKeys { namespaces.contains(it) }
        saveSettings(settings)
        bringUp(namespace)
        return getStatus()
    }
}
