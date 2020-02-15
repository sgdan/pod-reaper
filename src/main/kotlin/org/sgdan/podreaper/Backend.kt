package org.sgdan.podreaper

import io.fabric8.kubernetes.client.DefaultKubernetesClient
import io.fabric8.kubernetes.client.KubernetesClient
import io.micronaut.context.annotation.Factory
import io.micronaut.context.annotation.Parallel
import io.micronaut.context.annotation.Value
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.delay
import java.time.ZoneId
import javax.inject.Singleton

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

@Singleton
@Parallel // Don't wait until the first request before starting up!
class Backend(private val client: KubernetesClient,
              private val cfg: Config) {
    private val zoneId = ZoneId.of(cfg.zoneid)
    private val k8s = K8s(client, cfg.ignore, zoneId)

    private val manager =
            GlobalScope.run {
                managerActor(k8s, zoneId)
            }

    suspend fun getStatus(): Status =
            CompletableDeferred<Status>()
                    .also { manager.send(Manager.GetStatus(it)) }
                    .await()

    suspend fun setMemLimit(namespace: String, limit: Int): Status {
        manager.send(Manager.SetMemLimit(namespace, limit))
        return update(namespace)
    }

    suspend fun setStartHour(namespace: String, autoStartHour: Int?): Status {
        manager.send(Manager.SetStartHour(namespace, autoStartHour))
        return update(namespace)
    }

    suspend fun extend(namespace: String): Status {
        manager.send(Manager.Extend(namespace))
        return update(namespace)
    }

    private suspend fun update(namespace: String): Status {
        delay(30)
        manager.send(Manager.UpdateNamespace(namespace))
        delay(30) // allow time to update
        return getStatus()
    }
}
