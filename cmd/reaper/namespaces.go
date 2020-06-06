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
	for {
		select {
		case <-time.Tick(10 * time.Second):
			checkNamespaces(tickers, s)

		case ns := <-s.triggerNs:
			err := updateNamespace(ns, s)
			if err != nil {
				log.Printf("Unable to update namespace %v: %v", ns, err)
				if !s.cluster.getExists(ns) {
					tickers[ns] <- true
					delete(tickers, ns)
					log.Printf("Removing namespace: %v", ns)
					s.rmNsConfig <- ns
					s.rmNsStatus <- ns
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
	log.Printf("valid: %v", valid)

	// Start tickers for any new namespaces
	log.Printf("tickers: %v", tickers)
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
				log.Printf("Removed ticker for %v", name)
				ticker.Stop()
				return
			case <-ticker.C:
				s.triggerNs <- name
			}
		}
	}()
	log.Printf("created ticker for: %v", name)
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
	rq, err := checkQuota(name, s.cluster)
	// cluster.checkLimitRange(name)
	updated, err := loadNamespace(name, rq, s)
	if err == nil {
		s.updateNs <- updated
	}
	return err
}

func loadNamespace(name string, rq *v1.ResourceQuota, s state) (nsStatus, error) {
	memUsed := int64(0)
	memLimit := int64(10)
	if rq != nil {
		memUsed = rq.Status.Used.Memory().Value() / bytesInGi
		memLimit = rq.Spec.Hard.Memory().Value() / bytesInGi
	}
	cfg := s.getConfigFor(name)
	now := time.Now().In(&s.timeZone)
	lastScheduled := lastScheduled(cfg.AutoStartHour, now)
	lastStarted := max(cfg.LastStarted, lastScheduled)
	seconds := remainingSeconds(lastStarted, now.Unix())
	return nsStatus{
		Name:          name,
		HasDownQuota:  s.cluster.hasResourceQuota(name, downQuotaName),
		CanExtend:     seconds < (window-1)*60*60,
		MemUsed:       int(memUsed),
		MemLimit:      int(memLimit),
		AutoStartHour: cfg.AutoStartHour,
		Remaining:     remaining(seconds),
	}, nil
}

// Check if there's a quota for the namespace, create one if not
func checkQuota(namespace string, cluster k8s) (*v1.ResourceQuota, error) {
	quota, err := cluster.getResourceQuota(namespace, quotaName)
	if err != nil {
		log.Printf("Creating default quota for %v", namespace)
		value := resource.NewQuantity(defaultQuota, resource.Format("BinarySI"))
		quota, err = cluster.setResourceQuota(namespace, quotaName, *value)
		if err != nil {
			log.Printf("Unable to create quota for %v: %v", namespace, err)
		}
	}
	return quota, err
}
