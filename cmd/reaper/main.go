package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	// Examples for error handling:
	// - Use helper functions like e.g. errors.IsNotFound()
	// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
	namespace := "default"
	pod := "example-xxxxx"
	// _, err = clientset.CoreV1().Pods(namespace).Get(context.TODO(), pod, metav1.GetOptions{})
	_, err = clientset.CoreV1().Pods(namespace).Get(pod, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		fmt.Printf("Pod %s in namespace %s not found\n", pod, namespace)
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		fmt.Printf("Error getting pod %s in namespace %s: %v\n",
			pod, namespace, statusError.ErrStatus.Message)
	} else if err != nil {
		panic(err.Error())
	} else {
		fmt.Printf("Found pod %s in namespace %s\n", pod, namespace)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	http.HandleFunc("/hi", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hi")
	})

	// create status channel
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
			}
		}
	}()

	http.HandleFunc("/reaper/status", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, <-statusChannel)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))

}
