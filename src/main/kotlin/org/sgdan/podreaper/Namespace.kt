package org.sgdan.podreaper

import kotlinx.coroutines.*
import kotlinx.coroutines.channels.SendChannel
import kotlinx.coroutines.channels.actor
import mu.KotlinLogging
import java.lang.System.currentTimeMillis
import java.time.ZoneId
import java.time.ZonedDateTime

private val log = KotlinLogging.logger {}

sealed class Namespace() {
    object Update : Namespace()
    class SetMemLimit(val value: Int) : Namespace()
    class SetStartHour(val value: Int?) : Namespace()
    object Extend : Namespace()
}

fun CoroutineScope.namespaceActor(k8s: K8s,
                                  zoneId: ZoneId,
                                  cache: SendChannel<Cache>,
                                  settings: SendChannel<Settings>,
                                  name: String,
                                  manager: SendChannel<Manager>) = actor<Namespace> {
    var status: NamespaceStatus? = null

    for (msg in channel) when (msg) {
        is Namespace.Update -> try {
            if (k8s.getExists(name)) {

                // reap based on the previous status
                if (status != null) reap(status, zoneId, name, k8s, settings)

                // now update and send to cache
                val cfg = config(name, settings)
                status = k8s.loadNamespace(name, cfg.autoStartHour, cfg.lastStarted)
                cache.send(Cache.UpdateNamespace(name, status))
            } else {
                manager.send(Manager.RemoveNamespace(name))
            }
        } catch (e: Exception) {
            log.error { "Unable to update/load $name: ${e.message}" }
        }

        is Namespace.SetMemLimit -> try {
            k8s.setLimit(name, msg.value)
        } catch (e: Exception) {
            log.error { "Unable to set mem limit for $name: ${e.message}" }
        }

        is Namespace.SetStartHour -> settings.send(Settings.SetStartHour(name, msg.value))

        is Namespace.Extend -> try {
            k8s.bringUp(name)
            val started = currentTimeMillis() - 1
            settings.send(Settings.SetLastStarted(name, started))
        } catch (e: Exception) {
            log.error { "Unable to extend $name: ${e.message}" }
        }
    }
}.also {
    tick(5, it, Namespace.Update)
}

private suspend fun config(name: String, settings: SendChannel<Settings>) =
        CompletableDeferred<NamespaceConfig>()
                .also { settings.send(Settings.GetConfig(name, it)) }
                .await()

private suspend fun reap(status: NamespaceStatus,
                         zoneId: ZoneId,
                         name: String,
                         k8s: K8s,
                         settings: SendChannel<Settings>) = runBlocking {
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
        settings.send(Settings.SetLastStarted(name, started.toEpochSecond() * 1000))
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