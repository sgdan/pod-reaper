package main

import (
	"fmt"
	"time"
)

// From millis to seconds
func remainingSeconds(lastStarted int64, now int64) int64 {
	return max(lastStarted+window*60*60*1000-now, 0) / 1000
}

// Turn number of seconds into a readable string
func remainingTime(s int64) string {
	m := s / 60
	h := (m / 60) % window
	if m <= 0 || m >= window*60 {
		return ""
	}
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m%60)
	}
	return fmt.Sprintf("%dm", m%60)
}

func mostRecent(a *time.Time, b time.Time) time.Time {
	if a == nil {
		return b
	}
	if a.After(b) {
		return *a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// return same time as "now" but from most recent weekday, or nil
// if no start hour has been specified
func lastScheduled(startHour *int, now time.Time) *time.Time {
	if startHour != nil {
		last := weekday(withHour(now, *startHour))
		if last.After(now) {
			last = weekday(last.AddDate(0, 0, -1))
		}
		last = roundDownHour(last)
		return &last
	}
	return nil
}

func withHour(t time.Time, hour int) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), hour, t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

func roundDownHour(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

// return same time on most recent weekday
func weekday(now time.Time) time.Time {
	if isWeekend(now.Weekday()) {
		return weekday(now.AddDate(0, 0, -1))
	}
	return now
}

func isWeekend(day time.Weekday) bool {
	return day == time.Saturday || day == time.Sunday
}

func hoursFrom(earlier time.Time, later time.Time) int {
	return int(later.Sub(earlier).Hours())
}
