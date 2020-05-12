package org.sgdan.podreaper

import com.zakgof.actr.IActorRef
import java.lang.Thread.sleep

/**
 * The Ranger actor is responsible for ensuring each namespace
 * has a LimitRange which specifies default memory settings for
 * pods.
 */
class Ranger(private val k8s: K8s, private val cache: IActorRef<Cache>) {
    fun update() {
        cache.ask(Cache::getNamespaces) { namespaces ->
            namespaces.forEach {
                val hasLimitRange = k8s.getHasLimitRange(it.name)
                if (!hasLimitRange) k8s.setLimitRange(it.name)
                sleep(500)
            }
        }
    }
}
