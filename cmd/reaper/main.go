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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

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

	// Serve status from a channel
	emptyStatus := status{
		Clock:      "XX:YY GMT",
		Namespaces: []namespaceStatus{},
	}
	emptyStatusString, _ := json.Marshal(emptyStatus)
	statusChannel := make(chan string)
	go func() {
		current := string(emptyStatusString)
		for {
			select {
			case statusChannel <- current:
			case update := <-updates:
				nsStatuses[update.Name] = update

				// create sorted list of keys
				keys := make([]string, len(nsStatuses))
				i := 0
				for key := range nsStatuses {
					keys[i] = key
					i++
				}
				sort.Strings(keys)

				// add namespaces in sorted key order
				values := make([]namespaceStatus, len(nsStatuses))
				for index, key := range keys {
					values[index] = nsStatuses[key]
				}
				newStatus := status{
					Clock:      "newclock",
					Namespaces: values,
				}
				newStatusString, _ := json.Marshal(newStatus)
				current = string(newStatusString)
			}
		}
	}()

	// Update namespaces in the background
	go func() {
		nsTick := time.Tick(5 * time.Second)
		for {
			select {
			case <-nsTick:
				namespaces, _ := cluster.getNamespaces()
				for _, next := range namespaces {
					updates <- namespaceStatus{
						Name: next,
					}
				}
			}
		}
	}()

	http.HandleFunc("/reaper/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, <-statusChannel)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
