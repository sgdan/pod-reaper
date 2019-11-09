package org.sgdan.podreaper

import io.fabric8.kubernetes.client.DefaultKubernetesClient
import io.fabric8.kubernetes.client.KubernetesClient
import io.micronaut.context.annotation.Factory
import io.micronaut.context.annotation.Value
import io.micronaut.scheduling.annotation.Scheduled
import mu.KotlinLogging
import java.lang.System.currentTimeMillis
import java.time.ZoneId
import javax.inject.Singleton

const val LIMITS_MEMORY = "limits.memory"
const val MEMORY = "memory"
const val POD_REQUEST = "512Mi"
const val POD_LIMIT = "512Mi"
const val LIMIT_RANGE_NAME = "reaper-limit"
const val RESOURCE_QUOTA_NAME = "reaper-quota"
const val DEFAULT_QUOTA = "10Gi"
const val DOWN_QUOTA_NAME = "reaper-down-quota"
const val REAPER_NAMESPACE = "podreaper"
const val CONFIG_MAP_NAME = "podreaper-config"
const val CONFIG = "config"
const val WINDOW = 8 // Eight hour uptime window
const val CONTAINER = "Container"

private val log = KotlinLogging.logger {}

@Factory
class ClientFactory {
    @Singleton
    fun client() = DefaultKubernetesClient()
}

@Singleton
class Backend(private val client: KubernetesClient) {
    @Value("\${backend.kubernetes.ignore}")
    lateinit var ignore: Set<String>

    @Value("\${backend.kubernetes.zoneid}")
    lateinit var zoneid: String

    private var lastStatus = Status(ZoneId.systemDefault(), emptySet())

    @Synchronized
    @Scheduled(fixedDelay = "30s", initialDelay = "5s")
    fun reap() {
        try {
            lastStatus.namespaces.forEach { ns ->
                log.debug { "reaping ${ns.name}" }
                if (!ns.hasLimitRange) createLimitRange(client, ns.name)
                if (!ns.hasResourceQuota) createResourceQuota(client, ns.name, RESOURCE_QUOTA_NAME, DEFAULT_QUOTA)
                reap(client, lastStatus, ns)
            }
        } catch (e: Exception) {
            log.error(e) { "Unexpected error while reaping: $e" }
        }
    }

    @Synchronized
    fun getStatus() = read(client, Status(ZoneId.of(zoneid), ignore)).also {
        lastStatus = it
    }

    @Synchronized
    fun setMemLimit(namespace: String, limit: Int): Status {
        setLimit(client, namespace, limit)
        return getStatus()
    }

    @Synchronized
    fun setStartHour(namespace: String, autoStartHour: Int?): Status {
        val s = lastStatus.settings
        val newConfig = NamespaceConfig(autoStartHour, s[namespace]?.lastStarted ?: 0)
        saveSettings(client, s.plus(namespace to newConfig))
        return getStatus()
    }

    @Synchronized
    fun extend(namespace: String): Status {
        val s = lastStatus.settings
        bringUp(client, namespace)
        val newConfig = NamespaceConfig(s[namespace]?.autoStartHour, currentTimeMillis())
        saveSettings(client, s.plus(namespace to newConfig))
        return getStatus()
    }
}
