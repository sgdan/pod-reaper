package main

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type k8s struct {
	clientset kubernetes.Interface
}

func (o *k8s) getVersion() (string, error) {
	version, err := o.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", version), nil
}

func (o *k8s) getExists(namespace string) bool {
	_, err := o.clientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	return err == nil
}
