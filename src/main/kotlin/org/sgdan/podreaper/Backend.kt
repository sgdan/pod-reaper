package org.sgdan.podreaper

import com.zakgof.actr.Actr
import com.zakgof.actr.IActorRef
import io.fabric8.kubernetes.client.DefaultKubernetesClient
import io.fabric8.kubernetes.client.KubernetesClient
import io.micronaut.context.annotation.Factory
import io.micronaut.context.annotation.Parallel
import io.micronaut.context.annotation.Value
import mu.KotlinLogging
import java.time.ZoneId
import java.util.*
import java.util.concurrent.CompletableFuture
import javax.inject.Singleton
import kotlin.concurrent.timerTask

private val log = KotlinLogging.logger {}

@Factory
class ClientFactory {
    @Singleton
    fun client() = DefaultKubernetesClient()
}

class Config() {
    @Value("\${backend.kubernetes.ignore}")
    lateinit var ignore: Set<String>

    @Value("\${backend.kubernetes.zoneid}")
    lateinit var zoneid: String
}

/**
 * Convenient alternative to ask method. Returns a CompletableFuture wrapping the
 * result of a method call on the target actor.
 */
fun <R, T> IActorRef<T>.get(action: T.() -> R) =
        CompletableFuture<R>().also { job ->
            tell { job.complete(action(it)) }
        }

@Singleton
@Parallel // Don't wait until the first request before starting up!
class Backend(private val client: KubernetesClient,
              private val cfg: Config) {
    private val zoneId = ZoneId.of(cfg.zoneid)
    private val k8s = K8s(client, cfg.ignore, zoneId)
    private val system = Actr.newSystem("default")
    private val manager = system.actorOf { Manager(k8s, zoneId, system) }

    init {
        tick(30000) { manager.tell { it.update() } }
    }

    fun getStatus(): Status = manager.get { getStatus() }.join()

    fun setMemLimit(namespace: String, limit: Int): Status {
        manager.get { setMemLimit(namespace, limit) }.join()
        return manager.get { getStatus() }.join()
    }

    fun setStartHour(namespace: String, autoStartHour: Int?): Status {
        manager.get { setStartHour(namespace, autoStartHour) }
        return manager.get { getStatus() }.join()
    }

    fun extend(namespace: String): Status {
        manager.get { extend(namespace) }.join()
        return manager.get { getStatus() }.join()
    }
}

fun tick(period: Long, action: TimerTask.() -> Unit) {
    Timer().scheduleAtFixedRate(timerTask(action), 0, period)
}
