package org.sgdan.podreaper

import kotlinx.coroutines.*
import kotlinx.coroutines.channels.SendChannel
import kotlinx.coroutines.channels.actor
import mu.KotlinLogging
import java.time.ZoneId

private val log = KotlinLogging.logger {}

sealed class Manager() {
    object Update : Manager()
    class UpdateNamespace(val name: String) : Manager()
    class RemoveNamespace(val name: String) : Manager()
    class GetStatus(val job: CompletableDeferred<Status>) : Manager()
    class SetMemLimit(val namespace: String,
                      val value: Int) : Manager()

    class SetStartHour(val namespace: String,
                       val autoStartHour: Int?) : Manager()

    class Extend(val namespace: String) : Manager()
}

fun CoroutineScope.managerActor(k8s: K8s,
                                zoneId: ZoneId) = actor<Manager> {
    val namespaces = HashMap<String, SendChannel<Namespace>>()
    val settings: SendChannel<Settings> = settingsActor(k8s)
    val cache = cacheActor(zoneId)
    rangerActor(k8s, cache)

    for (msg in channel) when (msg) {
        is Manager.GetStatus -> cache.send(Cache.GetStatus(msg.job))

        is Manager.Update -> try {
            // create actors for new namespaces
            val live = k8s.getLiveNamespaces()
            val existing = namespaces.keys.toSet()
            live.minus(existing).forEach {
                namespaces[it] = namespaceActor(k8s, zoneId, cache, settings, it, channel)
            }
        } catch (e: Exception) {
            log.error { "Unable to update namespaces: ${e.message}" }
        }

        is Manager.RemoveNamespace -> {
            namespaces.remove(msg.name)?.close()
            cache.send(Cache.RemoveNamespace(msg.name))
            settings.send(Settings.RemoveConfigs(msg.name))
            log.info("Namespace ${msg.name} was removed")
        }

        is Manager.UpdateNamespace -> namespaces[msg.name]
                ?.send(Namespace.Update)

        is Manager.SetStartHour -> namespaces[msg.namespace]
                ?.send(Namespace.SetStartHour(msg.autoStartHour))

        is Manager.SetMemLimit -> namespaces[msg.namespace]
                ?.send(Namespace.SetMemLimit(msg.value))

        is Manager.Extend -> namespaces[msg.namespace]
                ?.send(Namespace.Extend)
    }
}.also {
    tick(30, it, Manager.Update)
}

/**
 * Trigger specified message at regular intervals to an actor
 */
fun <T> tick(seconds: Long, channel: SendChannel<T>, msg: T) {
    GlobalScope.launch {
        while (!channel.isClosedForSend) {
            channel.send(msg)
            delay(seconds * 1000)
        }
    }
}
