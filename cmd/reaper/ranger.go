package main

import "time"

func maintainLimitRanges(s state) {
	tick := time.Tick(s.Spec.RangerTick)
	for range tick {
		for _, state := range <-s.getStates {
			s.cluster.checkLimitRange(state.Name)
		}
	}
}
