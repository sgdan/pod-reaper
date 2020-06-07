package main

import "time"

func maintainLimitRanges(s state) {
	tick := time.Tick(60 * time.Second)
	for {
		select {
		case <-tick:
			for _, state := range <-s.getStates {
				s.cluster.checkLimitRange(state.Name)
			}
		}
	}
}
