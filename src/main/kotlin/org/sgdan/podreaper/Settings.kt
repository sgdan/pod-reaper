package org.sgdan.podreaper

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.channels.actor
import mu.KotlinLogging
import java.util.*

private val log = KotlinLogging.logger {}

/** Stored in config map in k8s cluster for each namespace */
data class NamespaceConfig(
        val autoStartHour: Int? = null,
        val lastStarted: Long = 0)

sealed class Settings() {
    class RemoveConfigs(val namespace: String) : Settings()
    class GetConfig(val namespace: String,
                    val job: CompletableDeferred<NamespaceConfig>) : Settings()

    class GetStartHour(val namespace: String,
                       val job: CompletableDeferred<Int?>) : Settings()

    class GetLastStarted(val namespace: String,
                         val job: CompletableDeferred<Long>) : Settings()

    class SetStartHour(val namespace: String,
                       val value: Int?) : Settings()

    class SetLastStarted(val namespace: String,
                         val value: Long) : Settings()
}

fun CoroutineScope.settingsActor(k8s: K8s) = actor<Settings> {
    val configs = TreeMap(k8s.getSettings())
    when (val n = configs.size) {
        0 -> log.warn { "Starting with blank configuration" }
        else -> log.info { "Loaded $n config values" }
    }

    for (msg in channel) when (msg) {
        is Settings.RemoveConfigs -> {
            configs.remove(msg.namespace)
            k8s.saveSettings(configs)
            log.info { "Removed config for ${msg.namespace}" }
        }
        is Settings.GetConfig -> msg.job.complete(
                configs[msg.namespace] ?: NamespaceConfig())

        is Settings.GetStartHour -> msg.job.complete(
                configs[msg.namespace]?.autoStartHour)
        is Settings.SetStartHour -> {
            val current = configs[msg.namespace] ?: NamespaceConfig()
            configs[msg.namespace] = current.copy(autoStartHour = msg.value)
            k8s.saveSettings(configs)
        }

        is Settings.GetLastStarted -> msg.job.complete(
                configs[msg.namespace]?.lastStarted ?: 0L)
        is Settings.SetLastStarted -> {
            val current = configs[msg.namespace] ?: NamespaceConfig()
            configs[msg.namespace] = current.copy(lastStarted = msg.value)
            k8s.saveSettings(configs)
        }
    }
}
