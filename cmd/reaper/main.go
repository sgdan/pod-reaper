package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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

/*
Specification contains default configuration for this app
that can be overridden using environment variables.
See https://github.com/kelseyhightower/envconfig

Overriding env values for backwards compatibility with the
previous Kotlin/Micronaut code.
*/
type Specification struct {
	IgnoredNamespaces []string `default:"kube-system,kube-public,kube-node-lease,podreaper,docker" envconfig:"ignored_namespaces"`
	ZoneID            string   `default:"UTC" envconfig:"zone_id"`
}

type namespaceConfig struct {
	AutoStartHour *int   `json:"autoStartHour"`
	LastStarted   uint64 `json:"lastStarted"`
}

type status struct {
	Clock      string            `json:"clock"`
	Namespaces []namespaceStatus `json:"namespaces"`
}

type namespaceStatus struct {
	// used by UI frontend
	Name         string `json:"name"`
	HasDownQuota bool   `json:"hasDownQuota"`
	// CanExtend     bool   `json:"canExtend"`
	MemUsed  int `json:"memUsed"`
	MemLimit int `json:"memLimit"`
	// AutoStartHour *int   `json:"autoStartHour"`
	// Remaining     string `json:"remaining"`

	// backend only
	// hasResourceQuota bool
	// lastScheduled ???
	// lastStarted uint64
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func main() {
	// load the configuration
	var s Specification
	err := envconfig.Process("reaper", &s)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Printf("Zone ID: %v", s.ZoneID)
	log.Printf("Ignored Namespaces: %v", s.IgnoredNamespaces)

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

	// Updated namespace statuses will be written to the update channel
	updates := make(chan namespaceStatus)
	nsNames := make(chan []string)
	tick := time.Tick(5 * time.Second) // update clock

	// Serve status from a channel
	emptyStatus := status{
		Clock:      "XX:YY GMT",
		Namespaces: []namespaceStatus{},
	}
	emptyStatusString, _ := json.Marshal(emptyStatus)
	statusChannel := make(chan string)
	go func() {
		current := string(emptyStatusString)
		now := time.Now().Format(timeFormat)
		nsStatuses := make(map[string]namespaceStatus)
		for {
			select {
			// send the current status to client
			case statusChannel <- current:

			case <-tick:
				newTime := time.Now().Format(timeFormat)
				if newTime != now {
					now = newTime
					current = updateStatus(nsStatuses, now)
					log.Printf("Updated time to: %v", now)
				}

			// update current status
			case update := <-updates:
				nsStatuses[update.Name] = update
				current = updateStatus(nsStatuses, now)

			// remove namespaces if required
			case valid := <-nsNames:
				for key := range nsStatuses {
					if !contains(valid, key) {
						log.Printf("Removing namespace %v", key)
						delete(nsStatuses, key)
					}
				}
			}
		}
	}()

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
					updateNamespace(next, updates, quota, cluster)
				}
			}
			log.Printf("valid: %v", valid)
			nsNames <- valid

			// sleep if updates finished within the interval
			elapsed := time.Now().Unix() - started
			remaining := max(namespaceInterval-elapsed, 1)
			time.Sleep(time.Duration(remaining) * time.Second)
		}
	}()

	http.HandleFunc("/reaper/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, <-statusChannel)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Update the JSON status to be returned to clients
func updateStatus(statuses map[string]namespaceStatus, clock string) string {
	// create sorted list of keys
	keys := make([]string, len(statuses))
	i := 0
	for key := range statuses {
		keys[i] = key
		i++
	}
	sort.Strings(keys)

	// add namespaces in sorted key order
	values := make([]namespaceStatus, len(statuses))
	for index, key := range keys {
		values[index] = statuses[key]
	}
	newStatus := status{
		Clock:      clock,
		Namespaces: values,
	}
	newStatusString, _ := json.Marshal(newStatus)
	return string(newStatusString)
}

func updateNamespace(name string, updates chan namespaceStatus, rq *v1.ResourceQuota, cluster k8s) {
	log.Printf("Updating namespace: %v", name)
	updated, err := loadNamespace(name, rq, cluster)
	if err != nil {
		log.Printf("Unable to load namespace %v: %v", name, err)
	} else {
		updates <- updated
	}
}

func loadNamespace(name string, rq *v1.ResourceQuota, cluster k8s) (namespaceStatus, error) {
	memUsed := int64(0)
	memLimit := int64(10)
	if rq != nil {
		memUsed = rq.Status.Used.Memory().Value() / bytesInGi
		memLimit = rq.Spec.Hard.Memory().Value() / bytesInGi
	}
	return namespaceStatus{
		Name:         name,
		HasDownQuota: cluster.hasResourceQuota(name, downQuotaName),
		MemUsed:      int(memUsed),
		MemLimit:     int(memLimit),
	}, nil
}

// Check if there's a quota for the namespace, create one if not
func checkQuota(namespace string, cluster k8s) (*v1.ResourceQuota, error) {
	quota, err := cluster.getResourceQuota(namespace, quotaName)
	if err != nil {
		log.Printf("Creating default quota for %v", namespace)
		// value := resource.NewScaledQuantity(defaultQuota, resource.Giga)
		value := resource.NewQuantity(defaultQuota, resource.Format("BinarySI"))
		quota, err = cluster.setResourceQuota(namespace, quotaName, *value)
		if err != nil {
			log.Printf("Unable to create quota for %v: %v", namespace, err)
		}
	}
	return quota, err
}
