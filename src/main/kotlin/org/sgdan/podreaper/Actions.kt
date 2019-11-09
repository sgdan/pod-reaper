package org.sgdan.podreaper

import com.fasterxml.jackson.databind.ObjectMapper
import io.fabric8.kubernetes.api.model.ConfigMapBuilder
import io.fabric8.kubernetes.api.model.LimitRangeBuilder
import io.fabric8.kubernetes.api.model.Quantity
import io.fabric8.kubernetes.client.KubernetesClient
import mu.KotlinLogging

private val log = KotlinLogging.logger {}
private val om = ObjectMapper()

/**
 * Remove any "reaper-down" resource quota so the namespace
 * will be running
 */
fun bringUp(client: KubernetesClient, namespace: String) {
    try {
        client.resourceQuotas().inNamespace(namespace).withName(DOWN_QUOTA_NAME).delete()
    } catch (e: Exception) {
        log.error(e) { "Unable to bring up $namespace" }
    }
}

fun bringDown(client: KubernetesClient, namespace: String) =
        createResourceQuota(client, namespace, DOWN_QUOTA_NAME, "0")

fun createResourceQuota(client: KubernetesClient, ns: String, name: String, limit: String) {
    try {
        client.resourceQuotas().inNamespace(ns).createNew()
                .withNewMetadata().withName(name).withNamespace(ns).endMetadata()
                .withNewSpec().withHard(mapOf(LIMITS_MEMORY to Quantity(limit)))
                .endSpec().done()
    } catch (e: Exception) {
        log.error(e) { "Unable to create resource quota $ns/$name/$limit" }
    }
}

fun setLimit(client: KubernetesClient, ns: String, limit: Int) {
    try {
        client.resourceQuotas().inNamespace(ns).withName(RESOURCE_QUOTA_NAME)
                .edit().editSpec()
                .addToHard(LIMITS_MEMORY, Quantity("${limit}Gi"))
                .endSpec().done()
    } catch (e: Exception) {
        log.error(e) { "Unable to set limit for $ns/$limit" }
    }
}

fun saveSettings(client: KubernetesClient, settings: Map<String, NamespaceConfig>) {
    try {
        val json = om.writeValueAsString(settings)
        val cm = ConfigMapBuilder()
                .withNewMetadata().withName(CONFIG_MAP_NAME).endMetadata()
                .addToData(CONFIG, json).build()
        client.configMaps().inNamespace((REAPER_NAMESPACE)).withName(CONFIG_MAP_NAME).createOrReplace(cm)
    } catch (e: Exception) {
        log.error(e) { "Unable to save settings" }
    }
}

fun reap(client: KubernetesClient, status: Status, ns: NamespaceStatus) {
    try {
        val started = mostRecent(ns.lastScheduled, toZDT(ns.lastStarted, status.zone))
        val shouldRun = hoursFrom(started, status.zdt) < 8

        // change up/down state
        if (!ns.hasDownQuota && !shouldRun) bringDown(client, ns.name)
        if (ns.hasDownQuota && shouldRun) bringUp(client, ns.name)

        // kill any pods that are running
        if (!shouldRun) client.pods().inNamespace(ns.name).delete()
    } catch (e: Exception) {
        log.error(e) { "Unable to reap pods in ${ns.name}" }
    }
}

fun createLimitRange(client: KubernetesClient, ns: String) {
    try {
        val lrb = LimitRangeBuilder()
                .withNewMetadata().withName(LIMIT_RANGE_NAME).endMetadata()
                .withNewSpec().addNewLimit()
                .withDefault(mapOf(MEMORY to Quantity(POD_LIMIT)))
                .withDefaultRequest(mapOf(MEMORY to Quantity(POD_REQUEST)))
                .withType(CONTAINER)
                .endLimit().endSpec().build()
        client.limitRanges().inNamespace(ns).withName(LIMIT_RANGE_NAME).createOrReplace(lrb)
    } catch (e: Exception) {
        log.error(e) { "Unable to create limit range in $ns" }
    }
}
