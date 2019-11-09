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
fun lastScheduled(startHour: Int?, now: ZonedDateTime) = startHour?.let {
    val start = weekday(now.withHour(it))
    if (now.isAfter(start)) start else weekday(start.minusDays(1))
} ?: toZDT(0, now.zone)
