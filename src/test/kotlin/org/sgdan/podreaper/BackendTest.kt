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

    private val backend: Backend by lazy {
        Backend(server.client).apply {
            ignore = setOf("kube-system", "kube-public")
            zoneid = "UTC"
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
        assertEquals(listOf("ns1", "ns2", "default"),
                backend.getStatus().namespaces.map { it.name })
    }

    @Test
    fun resourceQuota() {
        createNamespace("ns")
        assertFalse(backend.getStatus().namespaces[0].hasDownQuota)
        bringDown(server.client, "ns")
        assertTrue(backend.getStatus().namespaces[0].hasDownQuota)
        bringUp(server.client, "ns")
        assertFalse(backend.getStatus().namespaces[0].hasDownQuota)

        createResourceQuota(server.client, "ns", RESOURCE_QUOTA_NAME, "40Gi")
        assertEquals(40, backend.getStatus().namespaces[0].memLimit)
        backend.setMemLimit("ns", 30)
        assertEquals(30, backend.getStatus().namespaces[0].memLimit)
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

        assertEquals(thu8am, lastScheduled(8, thu9am))
        assertEquals(thu9am, lastScheduled(9, fri8am))
        assertEquals(fri9am, lastScheduled(9, sun10am))
        assertEquals(fri9am, lastScheduled(9, mon8am))
    }

    private fun remaining(lastStarted: Long, now: Long) =
            remainingTime(remainingSeconds(lastStarted, now))

    @Test
    fun calcRemaining() {
        val m = 60 * 1000
        val start = 1573261444114
        val stop = start + 8 * 60 * m // 8 hrs after start
        assertEquals("", remaining(0, stop))
        assertEquals("", remaining(stop - m + 1, start))
        assertEquals("1m", remaining(start, stop - m))
        assertEquals("5m", remaining(start, stop - 5 * m))
        assertEquals("10m", remaining(start, stop - 10 * m))
        assertEquals("1h 03m", remaining(start, stop - 63 * m))
        assertEquals("7h 59m", remaining(start, start + m))
        assertEquals("7h 59m", remaining(start, start + 1))
        assertEquals("", remaining(start, start))
        assertEquals("", remaining(start, start - 20 * m))
    }
}
