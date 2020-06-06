package main

import (
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Update namespaces in the background
func maintainNamespaces(s state) {
	// Each running namespace has an update ticker and close channel.
	// This map keeps track of the open tickers.
	tickers := map[string]bool{}

	checkNamespaces(tickers, s) // don't wait for tick for first one
	for {
		select {
		case <-time.Tick(10 * time.Second):
			checkNamespaces(tickers, s)

		case ns := <-s.rmNs:
			delete(tickers, ns)

		case ns := <-s.triggerNs:
			// log.Printf("Should update namespace: %v", ns)
			updateNamespace(ns, s)

			// default:
			// 	log.Printf("maintainNamespaces default!")
			// 	time.Sleep(1 * time.Second)
		}
	}
}

// Periodically check for new namespaces
func checkNamespaces(tickers map[string]bool, s state) {
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
			// quota, err := checkQuota(next, cluster)
			// if err != nil {
			// 	log.Printf("Unable to check quota for %v: %v", next, err)
			// }
			// tempAutoStartHour := 9
			// updateNamespace(next, &tempAutoStartHour, s.updateNs, quota, cluster, location)
		}
	}
	log.Printf("valid: %v", valid)

	// Start tickers for any new namespaces
	log.Printf("tickers: %v", tickers)
	for _, ns := range valid {
		if !tickers[ns] {
			createTickerFor(ns, s)
			tickers[ns] = true
		}
	}
}

// Create a ticker to trigger updates for a particular namespace
func createTickerFor(name string, s state) {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case removed := <-s.rmNs:
				log.Printf("Removed %v", removed)
				if removed == name {
					ticker.Stop()
					log.Printf("No longer updating namespace: %v", name)
					return
				}
			case <-ticker.C:
				s.triggerNs <- name
			}
		}
	}()
	log.Printf("created ticker for: %v", name)
}

func updateNamespace(name string, s state) {
	log.Printf("Updating namespace: %v", name)
	rq, err := checkQuota(name, s.cluster)

	// cluster.checkLimitRange(name)
	updated, err := loadNamespace(name, rq, s)
	if err != nil {
		log.Printf("Unable to load namespace %v: %v", name, err)
	} else {
		s.updateNs <- updated
	}
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
