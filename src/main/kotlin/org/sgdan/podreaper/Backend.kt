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

class Config() {
    @Value("\${backend.kubernetes.ignore}")
    lateinit var ignore: Set<String>

    @Value("\${backend.kubernetes.zoneid}")
    lateinit var zoneid: String

    var force = false
}

@Singleton
class Backend(private val client: KubernetesClient,
              private val cfg: Config) {
    private var current = Status(
            zone = ZoneId.of(cfg.zoneid),
            ignoredNamespaces = cfg.ignore,
            settings = readSettings(client))

    @Synchronized
    @Scheduled(fixedDelay = "10s")
    fun update() {
        try {
            current = updateNamespaces(current.copy(now = currentTimeMillis()), client, cfg.force)
        } catch (e: Exception) {
            log.error(e) { "Unable to update" }
        }
    }

    @Synchronized
    @Scheduled(fixedDelay = "10s", initialDelay = "5s")
    fun reap() {
        try {
            current = reapNamespaces(current, client)
        } catch (e: Exception) {
            log.error(e) { "Unable to reap" }
        }
    }

    @Synchronized
    fun getStatus(): Status = current

    @Synchronized
    fun setMemLimit(namespace: String, limit: Int): Status =
            try {
                setMemLimit(current, client, namespace, limit).also { current = it }
            } catch (e: Exception) {
                current.copy(error = "Unable to set start hour for $namespace: ${e.message}")
            }

    @Synchronized
    fun setStartHour(namespace: String, autoStartHour: Int?): Status =
            try {
                setStartHour(current, client, namespace, autoStartHour).also { current = it }
            } catch (e: Exception) {
                current.copy(error = "Unable to set start hour for $namespace: ${e.message}")
            }

    @Synchronized
    fun extend(namespace: String): Status =
            try {
                extend(current, client, namespace).also { current = it }
            } catch (e: Exception) {
                current.copy(error = "Unable to extend namespace: ${e.message}")
            }
}
