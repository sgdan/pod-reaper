package org.sgdan.podreaper

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.channels.SendChannel
import kotlinx.coroutines.channels.actor
import kotlinx.coroutines.delay

/**
 * The Ranger actor is responsible for ensuring each namespace
 * has a LimitRange which specifies default memory settings for
 * pods.
 */
sealed class Ranger() {
    object Update : Ranger()
}

fun CoroutineScope.rangerActor(k8s: K8s, cache: SendChannel<Cache>) = actor<Ranger> {
    for (msg in channel) when (msg) {
        is Ranger.Update -> {
            val namespaces = CompletableDeferred<List<NamespaceStatus>>()
                    .also { cache.send(Cache.GetNamespaces(it)) }
                    .await()
            namespaces.forEach {
                val hasLimitRange = k8s.getHasLimitRange(it.name)
                if (!hasLimitRange) k8s.setLimitRange(it.name)
                delay(500)
            }
        }
    }
}.also {
    tick(60, it, Ranger.Update)
}
