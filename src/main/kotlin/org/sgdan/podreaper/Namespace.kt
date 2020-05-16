package org.sgdan.podreaper

import com.zakgof.actr.IActorRef
import mu.KotlinLogging
import java.time.ZoneId
import java.time.ZonedDateTime
import java.util.concurrent.CompletableFuture

private val log = KotlinLogging.logger {}

class Namespace(private val k8s: K8s,
                private val zoneId: ZoneId,
                private val cache: IActorRef<Cache>,
                private val settings: IActorRef<Settings>,
                private val name: String,
                private val manager: IActorRef<Manager>) {

    private var status: NamespaceStatus? = null

    fun update() {
        try {
            if (k8s.getExists(name)) {
                // reap based on the previous status
                status?.let { reap(it, zoneId, name, k8s, settings) }

                // now update and send to cache
                quickUpdate()
            } else {
                manager.get { removeNamespace(name) }.join()
            }
        } catch (e: Exception) {
            log.error { "Unable to update/load $name: ${e.message}" }
        }
    }

    /**
     * Update and send to cache
     */
    fun quickUpdate() {
        val cfg = config(name)
        status = k8s.loadNamespace(name, cfg.autoStartHour, cfg.lastStarted)
        status?.let { current ->
            cache.get { updateNamespace(name, current) }.join()
        }
    }

    fun setMemLimit(value: Int) {
        try {
            k8s.setLimit(name, value)
            quickUpdate()
        } catch (e: Exception) {
            log.error { "Unable to set mem limit for $name: ${e.message}" }
        }
    }

    fun setStartHour(value: Int?) {
        try {
            settings.get { setStartHour(name, value) }.join()
            quickUpdate()
        } catch (e: Exception) {
            log.error { "Unable to set start hour for $name: ${e.message}" }
        }
    }

    fun extend() {
        try {
            k8s.bringUp(name)
            val started = System.currentTimeMillis() - 1
            settings.get { setLastStarted(name, started) }.join()
            quickUpdate()
        } catch (e: Exception) {
            log.error { "Unable to extend $name: ${e.message}" }
        }
    }

    private fun config(name: String) =
            CompletableFuture<NamespaceConfig>().also { job ->
                settings.tell { job.complete(it.getConfig(name)) }
            }.join()

    private fun reap(status: NamespaceStatus,
                     zoneId: ZoneId,
                     name: String,
                     k8s: K8s,
                     settings: IActorRef<Settings>) {
        if (!status.hasResourceQuota)
            k8s.setResourceQuota(name, RESOURCE_QUOTA_NAME, DEFAULT_QUOTA)

        val started = mostRecent(status.lastScheduled, toZDT(status.lastStarted, zoneId))
        val shouldRun = hoursFrom(started, ZonedDateTime.now(zoneId)) < 8

        // change up/down state
        if (!status.hasDownQuota && !shouldRun) try {
            k8s.bringDown(name)
        } catch (e: Exception) {
            log.error { "Unable to stop $name: ${e.message}" }
        }
        if (status.hasDownQuota && shouldRun) try {
            k8s.bringUp(name)
            settings.tell { it.setLastStarted(name, started.toEpochSecond() * 1000) }
        } catch (e: Exception) {
            log.error { "Unable to start $name: ${e.message}" }
        }

        // kill any pods that are running
        if (!shouldRun) try {
            k8s.deletePods(name)
        } catch (e: Exception) {
            log.error { "Unable to delete pods from $name: ${e.message}" }
        }
    }
}
