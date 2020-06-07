package main

import (
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Update namespaces in the background
func maintainNamespaces(s state) {
	// Each running namespace has an update ticker
	// This map holds close channels for stopping the tickers
	tickers := map[string](chan bool){}

	checkNamespaces(tickers, s) // don't wait for first tick
	tick := time.Tick(7 * time.Second)
	for {
		select {
		case <-tick:
			checkNamespaces(tickers, s)

		case ns := <-s.triggerNs:
			err := updateNamespace(ns, s)
			if err != nil {
				log.Printf("Unable to update namespace %v: %v", ns, err)
				if !s.cluster.getExists(ns) {
					log.Printf("Removing namespace: %v", ns)
					tickers[ns] <- true
					delete(tickers, ns)
					s.rmNamespace <- ns
				}
			}
		}
	}
}

// Periodically check for new namespaces
func checkNamespaces(tickers map[string](chan bool), s state) {
	namespaces, err := s.cluster.getNamespaces()
	if err != nil {
		log.Printf("Unable to retrieve namespaces: %v", err)
		return
	}

	// get the names of valid namespaces by removing the ignored ones
	valid := []string{}
	for _, next := range namespaces {
		if !contains(s.ignoredNamespaces, next) {
			valid = append(valid, next)
		}
	}

	// Start tickers for any new namespaces
	for _, ns := range valid {
		if _, ok := tickers[ns]; !ok {
			tickers[ns] = createTickerFor(ns, s)
		}
	}
}

// Create a ticker to trigger updates for a particular namespace
func createTickerFor(name string, s state) chan bool {
	ticker := time.NewTicker(5 * time.Second)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				s.triggerNs <- name
			}
		}
	}()
	return done
}

func updateNamespace(name string, s state) error {
	status, err := s.cluster.getStatusOf(name)
	if err != nil {
		return err
	}
	if status != "Active" {
		return fmt.Errorf("Namespace %v is %v", name, status)
	}
	rq, err := checkQuota(name, s)
	updated, err := loadNamespace(name, rq, s)
	if err == nil {
		s.updateNsState <- updated
	}
	return err
}

func loadNamespace(name string, rq *v1.ResourceQuota, s state) (nsState, error) {
	memUsed := int64(0)
	if rq != nil {
		memUsed = rq.Status.Used.Memory().Value() / bytesInGi
	}
	cfg := s.getConfigFor(name)
	now := time.Now().In(&s.timeZone)
	lastScheduled := lastScheduled(cfg.AutoStartHour, now)
	lastStarted := max(cfg.LastStarted, lastScheduled)
	seconds := remainingSeconds(lastStarted, now.Unix())
	return nsState{
		Name:          name,
		HasDownQuota:  s.cluster.hasResourceQuota(name, downQuotaName),
		MemUsed:       int(memUsed),
		Remaining:     remaining(seconds),
		LastScheduled: lastScheduled,
	}, nil
}

// Check if there's a quota for the namespace, create one if not
func checkQuota(ns string, s state) (*v1.ResourceQuota, error) {
	quota, err := s.cluster.getResourceQuota(ns, quotaName)
	if err != nil {
		log.Printf("Creating default quota for %v", ns)
		quota, err = setQuota(ns, defaultLimit, s)
		if err != nil {
			log.Printf("Unable to create quota for %v: %v", ns, err)
		}
	}
	limit := quota.Spec.Hard.Memory().Value()
	cfg := s.getConfigFor(ns)
	if cfg.Limit != int(limit/bytesInGi) {
		quota, err = setQuota(ns, int64(cfg.Limit), s)
		log.Printf("Updated %v limit to %v", ns, cfg.Limit)
	}
	return quota, err
}

func setQuota(ns string, limit int64, s state) (*v1.ResourceQuota, error) {
	value := resource.NewQuantity(limit*bytesInGi, resource.Format("BinarySI"))
	return s.cluster.setResourceQuota(ns, quotaName, *value)
}
