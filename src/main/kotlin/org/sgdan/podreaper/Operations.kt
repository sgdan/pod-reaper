/**
 * An "operation" takes the current status and makes some changes
 * before returning the new status.
 */
package org.sgdan.podreaper

import io.fabric8.kubernetes.client.KubernetesClient
import mu.KotlinLogging
import java.lang.System.currentTimeMillis
import kotlin.math.max

private val log = KotlinLogging.logger {}

fun setMemLimit(current: Status, client: KubernetesClient, name: String, limit: Int): Status {
    setLimit(client, name, limit)
    return loadNamespace(current, client, name)
}

fun setStartHour(current: Status, client: KubernetesClient, name: String, autoStartHour: Int?): Status {
    val s = current.settings
    val newConfig = NamespaceConfig(autoStartHour, s[name]?.lastStarted ?: 0)
    val newSettings = s.plus(name to newConfig)
    saveSettings(client, newSettings)
    return loadNamespace(current.copy(settings = newSettings), client, name)
}

fun extend(current: Status, client: KubernetesClient, name: String): Status {
    val s = current.settings
    bringUp(client, name)
    val newConfig = NamespaceConfig(s[name]?.autoStartHour, current.now - 1)
    val newSettings = s.plus(name to newConfig)
    saveSettings(client, newSettings)
    return loadNamespace(current.copy(settings = newSettings), client, name)
}

/** Update 20% of namespaces with each tick */
fun updateNamespaces(current: Status, client: KubernetesClient, force: Boolean = false): Status {
    val allNames = client.namespaces().list().items
            .map { it.metadata.name }
            .filter { !current.ignoredNamespaces.contains(it) }
    val loadedNames = current.namespaces.map { it.name }
    val toLoad = allNames.minus(loadedNames)
    val toUpdate = current.namespaces
            .sortedBy { it.lastRefreshed }
            .take(current.namespaces.size / 5)
            .map { it.name }
    return allNames.fold(current, { result, name ->
        if (force || toUpdate.contains(name) || toLoad.contains(name))
            loadNamespace(result, client, name)
        else result
    })
}

/** Reap 20% of namespaces with each tick */
fun reapNamespaces(current: Status, client: KubernetesClient): Status =
        current.namespaces
                .sortedBy { -it.lastRefreshed }
                .take(current.namespaces.size / 5)
                .fold(current, { result, namespace ->
                    reap(client, result, namespace)
                    loadNamespace(result, client, namespace.name)
                })

fun loadNamespace(current: Status, client: KubernetesClient, name: String): Status {
    val rq = readResourceQuota(client, name)
    val autoStartHour = current.settings[name]?.autoStartHour
    val lastScheduled = lastScheduled(autoStartHour, current.zdt)
    val lastScheduledMillis = lastScheduled.toEpochSecond() * 1000
    val lastStarted = max(current.settings[name]?.lastStarted ?: 0, lastScheduledMillis)
    val remaining = remainingSeconds(lastStarted, current.now)
    val loaded = NamespaceStatus(
            name,
            hasLimitRange(client, name),
            rq != null,
            hasDownQuota(client, name),
            remaining < (WINDOW - 1) * 60 * 60,
            toGigs(rq?.status?.used?.get(LIMITS_MEMORY)?.amount),
            toGigs(rq?.spec?.hard?.get(LIMITS_MEMORY)?.amount),
            autoStartHour,
            remainingTime(remaining),
            lastScheduled,
            lastStarted,
            lastRefreshed = current.now
    )
    return current.copy(namespaces = current.namespaces
            .filter { it.name != name }
            .plus(loaded)
            .sortedBy { it.name })
}

