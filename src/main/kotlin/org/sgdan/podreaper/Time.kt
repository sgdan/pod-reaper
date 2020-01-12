package org.sgdan.podreaper

import java.time.DayOfWeek
import java.time.Instant
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.temporal.ChronoUnit

private fun isWeekend(day: DayOfWeek) =
        setOf(DayOfWeek.SATURDAY, DayOfWeek.SUNDAY).contains(day)

/**
 * @return same time on most recent weekday
 */
private fun weekday(now: ZonedDateTime): ZonedDateTime =
        if (isWeekend(now.dayOfWeek)) weekday(now.minusDays(1)) else now

fun hoursFrom(earlier: ZonedDateTime, later: ZonedDateTime) =
        ChronoUnit.HOURS.between(earlier, later)

fun mostRecent(a: ZonedDateTime?, b: ZonedDateTime) = when {
    a == null -> b
    a.isAfter(b) -> a
    else -> b
}

fun toZDT(millis: Long, zone: ZoneId): ZonedDateTime =
        ZonedDateTime.ofInstant(Instant.ofEpochMilli(millis), zone)

/**
 * @return same time as "now" but from most recent weekday, or null
 *         if no start hour has been specified
 */
fun lastScheduled(startHour: Int?, now: ZonedDateTime): ZonedDateTime {
    val last = startHour?.let {
        val start = weekday(now.withHour(it))
        if (start.isAfter(now)) weekday(start.minusDays(1)) else start
    }
    return last?.withMinute(0)?.withSecond(0) ?: toZDT(0, now.zone)
}

fun remainingSeconds(lastStarted: Long, now: Long) =
        java.lang.Long.max(lastStarted + WINDOW * 60 * 60 * 1000 - now, 0) / 1000

fun remainingTime(remaining: Long): String {
    val m = remaining / 60
    val h = (m / 60) % WINDOW

    return when {
        m <= 0 || m >= WINDOW * 60 -> ""
        h > 0 -> "${h}h %02dm".format(m % 60)
        else -> "${m % 60}m"
    }
}
