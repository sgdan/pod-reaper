package org.sgdan.podreaper

import io.fabric8.kubernetes.api.model.PodBuilder
import io.fabric8.kubernetes.client.server.mock.KubernetesServer
import mu.KotlinLogging
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import java.time.ZoneId

private val log = KotlinLogging.logger {}

class K8sTest {
    @Rule
    @JvmField
    public val server = KubernetesServer(true, true)

    private val cfg = Config().apply {
        ignore = setOf("kube-system", "kube-public")
        zoneid = "UTC"
    }

    private val k8s: K8s by lazy {
        K8s(server.client, cfg.ignore, ZoneId.of(cfg.zoneid))
    }

    private fun createNamespace(name: String) =
            server.client.namespaces().createNew()
                    .withNewMetadata().withName(name).endMetadata().done()

    /**
     * see https://github.com/fabric8io/kubernetes-client#mocking-kubernetes
     */
    private fun createPod(name: String, namespace: String) =
            server.client.pods().inNamespace(namespace).create(
                    PodBuilder().withNewMetadata()
                            .withName(name)
                            .endMetadata().build())

    private fun countPods(namespace: String) =
            server.client.pods().inNamespace(namespace).list().items.size

    @Test
    fun getNamespaces() {
        assertFalse(k8s.getExists("ns1"))
        listOf("ns1", "ns2", "kube-system", "kube-public", "default")
                .forEach { createNamespace(it) }
        assertEquals(setOf("default", "ns1", "ns2"), k8s.getLiveNamespaces())
        assertTrue(k8s.getExists("ns1"))
    }

    @Test
    fun resourceQuota() {
        createNamespace("ns")
        assertFalse(k8s.getHasDownQuota("ns"))
        k8s.bringDown("ns")
        assertTrue(k8s.getHasDownQuota("ns"))
        k8s.bringUp("ns")
        assertFalse(k8s.getHasDownQuota("ns"))

        k8s.setResourceQuota("ns", RESOURCE_QUOTA_NAME, "40Gi")
        val loaded1 = k8s.loadNamespace("ns", null, 0)
        assertEquals(40, loaded1.memLimit)
        k8s.setLimit("ns", 30)
        val loaded2 = k8s.loadNamespace("ns", null, 0)
        assertEquals(30, loaded2.memLimit)
        val rq = k8s.getResourceQuota("ns")
        assertEquals("30Gi", rq?.spec?.hard?.get(LIMITS_MEMORY)?.amount)
    }

    @Test
    fun settings() {
        assertTrue(k8s.getSettings().isEmpty())
        val cfg = mapOf("one" to NamespaceConfig(null, 24),
                "two" to NamespaceConfig(23, 2394827398743))
        k8s.saveSettings(cfg)
        assertEquals(cfg, k8s.getSettings())
    }

    @Test
    fun limitRange() {
        createNamespace("testLR")
        assertFalse(k8s.getHasLimitRange("testLR"))
        k8s.setLimitRange("testLR")
        assertTrue(k8s.getHasLimitRange("testLR"))
        val limit = k8s.getLimitRange("testLR")
                .get().spec.limits[0]
        assertEquals(POD_LIMIT, limit.default?.get(MEMORY)?.amount)
        assertEquals(POD_REQUEST, limit.defaultRequest?.get(MEMORY)?.amount)
    }

    /**
     * Check if pods can be deleted as expected. Note that in a real
     * cluster permissions will be required (see deploy.yaml in the root
     * of this project for examples).
     */
    @Test
    fun pods() {
        assertEquals(0, countPods("ns1"))
        listOf("one", "two", "three").forEach { createPod(it, "ns1") }
        assertEquals(3, countPods("ns1"))
        k8s.deletePods("ns1")
        assertEquals(0, countPods("ns1"))
    }
}
