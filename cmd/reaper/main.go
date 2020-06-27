package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const timeFormat = "15:04 MST"
const namespaceInterval = 5 // target interval between namespace updates
const quotaName = "reaper-quota"
const downQuotaName = "reaper-down-quota"
const bytesInGi = 1024 * 1024 * 1024
const defaultLimit = 10
const limitRangeName = "reaper-limit"
const podRequest = "512Mi"
const podLimit = "512Mi"
const window = 8 // hours in uptime window
const configMapName = "podreaper-goconfig"

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

	var clientset *kubernetes.Clientset
	if spec.InCluster {
		log.Printf("Using in-cluster configuration")
		clientset = initInCluster()
	} else {
		log.Printf("Using out-of-cluster configuration")
		clientset = initOutOfCluster()
	}

	s := newState(spec, *location, k8s{clientset: clientset})
	go maintainStatus(s)
	go maintainNamespaces(s)
	go maintainLimitRanges(s)
	go reap(s)

	// serve latest cached JSON status to clients
	http.HandleFunc("/reaper/status", func(w http.ResponseWriter, r *http.Request) {
		if setHeaders(r.Method, spec, w, r) {
			fmt.Fprint(w, <-s.getStatus)
		}
	})
	http.HandleFunc("/reaper/setMemLimit", func(w http.ResponseWriter, r *http.Request) {
		if setHeaders(r.Method, spec, w, r) {
			decoder := json.NewDecoder(r.Body)
			var lr limitRequest
			err := decoder.Decode(&lr)
			if err == nil {
				cfg := s.getConfigFor(lr.Namespace)
				cfg.Limit = lr.Limit
				s.updateNsConfig <- cfg
			} else {
				log.Printf("Unable to set start hour: %v", err)
			}
			fmt.Fprint(w, <-s.getStatus)
		}
	})
	http.HandleFunc("/reaper/setStartHour", func(w http.ResponseWriter, r *http.Request) {
		if setHeaders(r.Method, spec, w, r) {
			decoder := json.NewDecoder(r.Body)
			var sr startRequest
			err := decoder.Decode(&sr)
			if err == nil {
				cfg := s.getConfigFor(sr.Namespace)
				cfg.AutoStartHour = sr.StartHour
				s.updateNsConfig <- cfg
			} else {
				log.Printf("Unable to set start hour: %v", err)
			}
			fmt.Fprint(w, <-s.getStatus)
		}
	})
	http.HandleFunc("/reaper/extend", func(w http.ResponseWriter, r *http.Request) {
		if setHeaders(r.Method, spec, w, r) {
			decoder := json.NewDecoder(r.Body)
			var sr startRequest
			err := decoder.Decode(&sr)
			if err == nil {
				cfg := s.getConfigFor(sr.Namespace)
				cfg.LastStarted = time.Now().Unix() - 1
				s.updateNsConfig <- cfg
			} else {
				log.Printf("Unable to extend namespace: %v", err)
			}
			fmt.Fprint(w, <-s.getStatus)
		}
	})

	// serve the front end static files
	if spec.StaticFiles != "" {
		fs := http.FileServer(http.Dir(spec.StaticFiles))
		http.Handle("/", fs)
	}

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Exit: %v", err)
	}
}

// Set headers based on CORS configuration
// Return true if content can be returned (i.e. false for OPTIONS response)
func setHeaders(method string, spec Specification, w http.ResponseWriter, r *http.Request) bool {
	h := w.Header()
	if spec.CorsEnabled {
		for _, origin := range spec.CorsOrigins {
			h.Set("Access-Control-Allow-Origin", origin)
		}
		if r.Method == "OPTIONS" {
			h.Set("Access-Control-Allow-Methods", "POST")
			h.Set("Access-Control-Allow-Headers", "content-type")
			return false
		}
	} else {
		h.Set("Content-Type", "application/json")
	}
	return true
}

// Use in-cluster config to connect to k8s api
// see https://github.com/kubernetes/client-go/blob/master/examples/in-cluster-client-configuration/main.go
func initInCluster() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

// Use local kubeconfig to connect to k8s api
// see https://github.com/kubernetes/client-go/tree/master/examples/out-of-cluster-client-configuration
func initOutOfCluster() *kubernetes.Clientset {
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
	return clientset
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
