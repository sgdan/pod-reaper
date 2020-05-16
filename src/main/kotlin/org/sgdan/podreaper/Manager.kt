package org.sgdan.podreaper

import com.zakgof.actr.Actr
import com.zakgof.actr.IActorRef
import com.zakgof.actr.IActorSystem
import mu.KotlinLogging
import java.time.ZoneId

private val log = KotlinLogging.logger {}

class Manager(private val k8s: K8s,
              private val zoneId: ZoneId,
              private val system: IActorSystem) {
    private val namespaces = HashMap<String, IActorRef<Namespace>>()
    private val settings: IActorRef<Settings> = system.actorOf { Settings(k8s) }
    private val cache = system.actorOf { Cache(zoneId) }
    private val ranger = system.actorOf { Ranger(k8s, cache) }

    init {
        tick(60000) { ranger.tell { it.update() } }
    }

    fun getStatus(): Status = cache.get { getStatus() }.join()

    fun update() = try {
        // create actors for new namespaces
        val live = k8s.getLiveNamespaces()
        val existing = namespaces.keys.toSet()
        live.minus(existing).forEach { name ->
            namespaces[name] = system.actorOf {
                Namespace(k8s, zoneId, cache, settings, name, Actr.current())
            }
            tick(5000) { namespaces[name]?.tell { it.update() } }
        }
    } catch (e: Exception) {
        log.error { "Unable to update namespaces: ${e.message}" }
    }

    fun removeNamespace(name: String) {
        namespaces.remove(name)?.close()
        cache.tell { it.removeNamespace(name) }
        settings.tell { it.removeConfigs(name) }
        log.info("Namespace $name was removed")
    }

    fun setStartHour(namespace: String, autoStartHour: Int?) {
        namespaces[namespace]?.get { setStartHour(autoStartHour) }?.join()
    }

    fun setMemLimit(namespace: String, value: Int) {
        namespaces[namespace]?.get { setMemLimit(value) }?.join()
    }

    fun extend(namespace: String) {
        namespaces[namespace]?.get { extend() }?.join()
    }
}
