package main

import (
	"fmt"
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
