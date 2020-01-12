package org.sgdan.podreaper

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.channels.actor
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.util.*

private val formatter: DateTimeFormatter =
        DateTimeFormatter.ofPattern("HH:mm z")

sealed class Cache() {
    class GetStatus(val job: CompletableDeferred<Status>) : Cache()
    class GetNamespaces(val job: CompletableDeferred<List<NamespaceStatus>>) : Cache()
    class RemoveNamespace(val name: String) : Cache()
    class UpdateNamespace(val name: String,
                          val value: NamespaceStatus) : Cache()
}

fun CoroutineScope.cacheActor(zoneId: ZoneId) = actor<Cache> {
    val namespaces = TreeMap<String, NamespaceStatus>()

    for (msg in channel) when (msg) {
        is Cache.GetStatus -> {
            val clock = ZonedDateTime.now(zoneId).format(formatter)
            msg.job.complete(Status(clock, namespaces.values.toList()))
        }
        is Cache.GetNamespaces -> {
            msg.job.complete(namespaces.values.toList())
        }
        is Cache.RemoveNamespace -> namespaces.remove(msg.name)
        is Cache.UpdateNamespace -> namespaces[msg.name] = msg.value
    }
}
