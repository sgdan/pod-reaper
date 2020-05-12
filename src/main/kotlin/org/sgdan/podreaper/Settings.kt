package org.sgdan.podreaper

import mu.KotlinLogging
import java.util.*

private val log = KotlinLogging.logger {}

/** Stored in config map in k8s cluster for each namespace */
data class NamespaceConfig(
        val autoStartHour: Int? = null,
        val lastStarted: Long = 0)

class Settings(private val k8s: K8s) {
    private val configs = TreeMap(k8s.getSettings())

    init {
        when (val n = configs.size) {
            0 -> log.warn { "Starting with blank configuration" }
            else -> log.info { "Loaded $n config values" }
        }
    }

    fun removeConfigs(namespace: String) {
        configs.remove(namespace)
        k8s.saveSettings(configs)
        log.info { "Removed config for $namespace" }
    }

    fun getConfig(namespace: String) = configs[namespace] ?: NamespaceConfig()

    fun getStartHour(namespace: String) = configs[namespace]?.autoStartHour

    fun setStartHour(namespace: String, value: Int?) {
        val current = configs[namespace] ?: NamespaceConfig()
        configs[namespace] = current.copy(autoStartHour = value)
        k8s.saveSettings(configs)
    }

    fun getLastStarted(namespace: String) = configs[namespace]?.lastStarted ?: 0L

    fun setLastStarted(namespace: String, value: Long) {
        val current = configs[namespace] ?: NamespaceConfig()
        configs[namespace] = current.copy(lastStarted = value)
        k8s.saveSettings(configs)
    }
}
