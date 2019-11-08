package org.sgdan.podreaper

import io.fabric8.kubernetes.client.server.mock.KubernetesServer
import mu.KotlinLogging
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import java.time.ZonedDateTime

private val log = KotlinLogging.logger {}

class K8sBackendTest {
    @Rule
    @JvmField
    public val server = KubernetesServer(true, true)

    private val backend: K8sBackend by lazy {
        K8sBackend(server.client).apply {
            ignore = setOf("kube-system", "kube-public")
        }
    }

    private fun createNamespace(name: String) {
        server.client.namespaces().createNew()
                .withNewMetadata().withName(name).endMetadata().done()
    }

    @Test
    fun getNamespaces() {
        listOf("ns1", "ns2", "kube-system", "kube-public", "default")
                .forEach { createNamespace(it) }
        assertEquals(listOf("ns1", "ns2", "default"), backend.getNamespaces())
    }

    @Test
    fun resourceQuota() {
        "ns".let {
            createNamespace(it)
            assertNotNull(server.client.namespaces().withName(it).get())
            assertTrue(backend.isUp(it))
            backend.createResourceQuota(it, "reaper-down", 0)
            assertFalse(backend.isUp(it))
            backend.bringUp(it)
            assertTrue(backend.isUp(it))
            backend.bringDown(it)
            assertFalse(backend.isUp(it))
        }
    }

    @Test
    fun setLimit() {
        "ns".let {
            createNamespace(it)
            try {
                backend.setLimit(it, "rqname", 4)
                fail("Should throw ReaperException")
            } catch (r: ReaperException) {
                // expected
            }
            backend.createResourceQuota(it, "rqname", 7)
            assertEquals(Pair(7, 0), backend.getLimitUsed(it, "rqname"))
        }
    }

    private fun zdt(day: String, hour: String) = ZonedDateTime.parse("2019-10-${day}T${hour}:00Z")

    @Test
    fun schedules() {
        val thu8am = zdt("24", "08")
        val thu9am = zdt("24", "09")
        val fri8am = zdt("25", "08")
        val fri9am = zdt("25", "09")
        val sun10am = zdt("27", "10")
        val mon8am = zdt("28", "08")

        assertEquals(thu8am, backend.lastScheduled(8, thu9am))
        assertEquals(thu9am, backend.lastScheduled(9, fri8am))
        assertEquals(fri9am, backend.lastScheduled(9, sun10am))
        assertEquals(fri9am, backend.lastScheduled(9, mon8am))
    }
}
