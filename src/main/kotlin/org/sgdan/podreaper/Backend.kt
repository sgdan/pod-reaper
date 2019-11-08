package org.sgdan.podreaper

import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter

data class NamespaceStatus(val name: String,
                           val up: Boolean,
                           val memUsed: Int,
                           val memLimit: Int,
                           val startHour: Int?,
                           val stopTime: Long)

data class Status(val namespaces: List<NamespaceStatus>,
                  val time: String = ZonedDateTime.now().format(formatter))

val formatter: DateTimeFormatter = DateTimeFormatter.ofPattern("HH:mm z")

interface Backend {
    fun getStatus(): Status
    fun setMemLimit(namespace: String, limit: Int): Status
    fun setStartHour(namespace: String, startHour: Int?): Status
    fun extend(namespace: String): Status
}
