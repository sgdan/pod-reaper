package main

import "time"

func maintainLimitRanges(s state) {
	tick := time.Tick(s.Spec.RangerTick)
	for {
		select {
		case <-tick:
			for _, state := range <-s.getStates {
				s.cluster.checkLimitRange(state.Name)
			}
		}
	}
}
