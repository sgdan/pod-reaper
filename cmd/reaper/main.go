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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const timeFormat = "15:04 MST"
const namespaceInterval = 5 // target interval between namespace updates

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
	Name string `json:"name"`
	// HasDownQuota  bool   `json:"hasDownQuota"`
	// CanExtend     bool   `json:"canExtend"`
	// MemUsed       int    `json:"memUsed"`
	// MemLimit      int    `json:"memLimit"`
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
	nsStatuses := make(map[string]namespaceStatus)
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
			}
		}
	}()

	// Update namespaces in the background
	go func() {
		for {
			started := time.Now().Unix()
			namespaces, _ := cluster.getNamespaces()
			log.Printf("Got namespaces: %v", namespaces)
			for _, next := range namespaces {
				if !contains(s.IgnoredNamespaces, next) {
					updateNamespace(next, updates)
				}
			}
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

func updateNamespace(name string, updates chan namespaceStatus) {
	log.Printf("Updating namespace: %v", name)
	updates <- namespaceStatus{
		Name: name,
	}
}
