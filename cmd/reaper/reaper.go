package main

import (
	"log"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

func reap(s state) {
	tick := time.Tick(21 * time.Second)
	for {
		select {
		case <-tick:
			cfgs := s.configMap()
			for _, state := range <-s.getStates {
				ns := state.Name
				cfg := cfgs[ns]
				started := max(state.LastScheduled, cfg.LastStarted)
				shouldRun := hoursFrom(started, time.Now().Unix()) < window

				// change up/down state
				if !state.HasDownQuota && !shouldRun {
					bringDown(ns, s)
				}
				if state.HasDownQuota && shouldRun {
					bringUp(ns, s)
				}

				// kill any pods that are running
				if !shouldRun {
					err := s.cluster.deletePods(ns)
					if err != nil {
						log.Printf("Unable to delete pods in %v: %v", ns, err)
					}
				}
			}
		}
	}
}

func bringUp(ns string, s state) {
	if s.cluster.hasResourceQuota(ns, downQuotaName) {
		err := s.cluster.removeResourceQuota(ns, downQuotaName)
		if err != nil {
			log.Printf("Unable to bring up %v: %v", ns, err)
		} else {
			log.Printf("Bringing up %v", ns)
		}
	}
}

func bringDown(ns string, s state) {
	if !s.cluster.hasResourceQuota(ns, downQuotaName) {
		value := resource.NewQuantity(0, resource.Format("BinarySI"))
		_, err := s.cluster.setResourceQuota(ns, downQuotaName, *value)
		if err != nil {
			log.Printf("Unable to bring down %v: %v", ns, err)
		} else {
			log.Printf("Bringing down %v", ns)
		}
	}
}
