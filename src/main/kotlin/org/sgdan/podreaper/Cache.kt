package org.sgdan.podreaper

import mu.KotlinLogging
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.util.*

private val log = KotlinLogging.logger {}

private val formatter: DateTimeFormatter =
        DateTimeFormatter.ofPattern("HH:mm z")

class Cache(private val zoneId: ZoneId) {
    private val namespaces = TreeMap<String, NamespaceStatus>()

    fun getStatus(): Status {
        val clock = ZonedDateTime.now(zoneId).format(formatter)
        return Status(clock, namespaces.values.toList())
    }

    fun getNamespaces() = namespaces.values.toList()

    fun removeNamespace(name: String) {
        namespaces.remove(name)
    }

    fun updateNamespace(name: String, value: NamespaceStatus) {
        namespaces[name] = value
    }
}
