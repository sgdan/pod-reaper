package org.sgdan.podreaper

import java.time.ZonedDateTime

/**
 * A read-only snapshot of the relevant data from the kubernetes
 * cluster describing the namespaces we're interested in.
 */
data class Status(
        val clock: String = "",
        val namespaces: List<NamespaceStatus> = emptyList())

data class NamespaceStatus(
        // used by UI frontend
        val name: String,
        val hasDownQuota: Boolean, // the resource quote with zero limit used to disable namespace
        val canExtend: Boolean,
        val memUsed: Int,
        val memLimit: Int,
        val autoStartHour: Int?,
        val remaining: String,

        // only backend
        val hasResourceQuota: Boolean,
        val lastScheduled: ZonedDateTime?,
        val lastStarted: Long)
