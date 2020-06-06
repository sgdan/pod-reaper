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
		log.Fatalf("Can't load environment vars: %v", err)
	}
	log.Printf("Zone ID: %v", spec.ZoneID)
	log.Printf("Ignored Namespaces: %v", spec.IgnoredNamespaces)
	location, err := time.LoadLocation(spec.ZoneID)
	if err != nil {
		log.Fatalf("Invalid Zone ID: %v", err)
	}

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

	s := newState(*location, spec.IgnoredNamespaces, k8s{clientset: clientset})
	go maintainConfigs(s)
	go maintainStatus(s)
	go maintainNamespaces(s)

	// serve latest cached JSON status to clients
	http.HandleFunc("/reaper/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if spec.CorsEnabled {
			for _, origin := range spec.CorsOrigins {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}
		fmt.Fprint(w, <-s.getStatus)
	})

	log.Fatalf("Exit: %v", http.ListenAndServe(":8080", nil))
}
