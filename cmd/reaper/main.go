package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kelseyhightower/envconfig"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const timeFormat = "15:04 MST"
const namespaceInterval = 5 // target interval between namespace updates
const quotaName = "reaper-quota"
const downQuotaName = "reaper-down-quota"
const bytesInGi = 1024 * 1024 * 1024
const defaultQuota = 10 * bytesInGi
const limitRangeName = "reaper-limit"
const podRequest = "512Mi"
const podLimit = "512Mi"
const window = 8 // hours in uptime window
const configMapName = "podreaper-goconfig"

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func main() {
	// load the configuration
	var spec Specification
	err := envconfig.Process("reaper", &spec)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Printf("Zone ID: %v", spec.ZoneID)
	log.Printf("Ignored Namespaces: %v", spec.IgnoredNamespaces)
	location, err := time.LoadLocation(spec.ZoneID)
	if err != nil {
		log.Fatalf("Invalid Zone ID: %v", err)
	}
	log.Printf("Time Zone: %v", location)

	// k8s connection info
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	cluster := k8s{
		clientset: clientset,
	}

	// Create state
	s := newState(*location, spec.IgnoredNamespaces)
	// updates := make(chan namespaceStatus)
	// nsNames := make(chan []string)

	// Serve status from a channel
	// emptyStatusString, _ := json.Marshal(emptyStatus)
	// statusChannel := make(chan string)
	go maintainStatus(s)
	// go func() {
	// 	settings, err := cluster.getSettings()
	// 	if err != nil {
	// 		settings = map[string]namespaceConfig{}
	// 	}
	// 	log.Printf("settings, err: %v, %v", settings, err)
	// 	current := string(emptyStatusString)
	// 	now := time.Now().In(location).Format(timeFormat)
	// 	nsStatuses := make(map[string]namespaceStatus)
	// 	for {
	// 		select {
	// 		// send the current status to client
	// 		case statusChannel <- current:

	// 		case <-tick:
	// 			newTime := time.Now().In(location).Format(timeFormat)
	// 			if newTime != now {
	// 				now = newTime
	// 				current = updateStatus(nsStatuses, now)
	// 				log.Printf("Updated time to: %v", now)
	// 			}

	// 		// update current status
	// 		case update := <-updates:
	// 			nsStatuses[update.Name] = update
	// 			current = updateStatus(nsStatuses, now)

	// 		// remove namespaces if required
	// 		case valid := <-nsNames:
	// 			for key := range nsStatuses {
	// 				if !contains(valid, key) {
	// 					log.Printf("Removing namespace %v", key)
	// 					delete(nsStatuses, key)
	// 				}
	// 			}
	// 		}
	// 	}
	// }()

	// Update namespaces in the background
	go func() {
		for {
			// create list of valid namespace names
			started := time.Now().Unix()
			namespaces, _ := cluster.getNamespaces()
			log.Printf("Got namespaces: %v", namespaces)
			valid := []string{}
			for _, next := range namespaces {
				if !contains(s.IgnoredNamespaces, next) {
					valid = append(valid, next)
					quota, err := checkQuota(next, cluster)
					if err != nil {
						log.Printf("Unable to check quota for %v: %v", next, err)
					}
					tempAutoStartHour := 9
					updateNamespace(next, &tempAutoStartHour, s.updateNs, quota, cluster, location)
				}
			}
			log.Printf("valid: %v", valid)
			// nsNames <- valid

			// sleep if updates finished within the interval
			elapsed := time.Now().Unix() - started
			remaining := max(namespaceInterval-elapsed, 1)
			time.Sleep(time.Duration(remaining) * time.Second)
		}
	}()

	http.HandleFunc("/reaper/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, <-s.getStatus)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func updateNamespace(name string, autoStartHour *int, updates chan nsStatus, rq *v1.ResourceQuota, cluster k8s, zone *time.Location) {
	log.Printf("Updating namespace: %v", name)
	cluster.checkLimitRange(name)
	updated, err := loadNamespace(name, autoStartHour, rq, cluster, zone)
	if err != nil {
		log.Printf("Unable to load namespace %v: %v", name, err)
	} else {
		updates <- updated
	}
}

func loadNamespace(name string, autoStartHour *int, rq *v1.ResourceQuota, cluster k8s, zone *time.Location) (nsStatus, error) {
	memUsed := int64(0)
	memLimit := int64(10)
	if rq != nil {
		memUsed = rq.Status.Used.Memory().Value() / bytesInGi
		memLimit = rq.Spec.Hard.Memory().Value() / bytesInGi
	}
	// now := time.Now().In(zone)
	// lastScheduled := lastScheduled(autoStartHour, now)
	// lastStarted := max(prevStarted, lastScheduledMillis)
	// remaining := remainingSeconds(lastStarted, now)
	return nsStatus{
		Name:         name,
		HasDownQuota: cluster.hasResourceQuota(name, downQuotaName),
		// CanExtend:    remaining < (window-1)*60*60,
		MemUsed:  int(memUsed),
		MemLimit: int(memLimit),
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
