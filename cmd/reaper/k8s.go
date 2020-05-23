package main

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type k8s struct {
	clientset kubernetes.Interface
}

type namespaceConfig struct {
	autoStartHour *int
	lastStarted   uint64
}

func (o *k8s) getVersion() (string, error) {
	version, err := o.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", version), nil
}

func (o *k8s) createNamespace(name string) {
	nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	o.clientset.CoreV1().Namespaces().Create(nsSpec)
}

func (o *k8s) getExists(namespace string) bool {
	_, err := o.clientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	return err == nil
}

func (o *k8s) getConfigMap(name string) (*v1.ConfigMap, error) {
	cm, err := o.clientset.CoreV1().ConfigMaps("podreaper").
		Get(name, metav1.GetOptions{})
	return cm, err
}

func (o *k8s) getSettings() (string, error) {
	cm, err := o.getConfigMap("podreaper-config")
	if err != nil {
		return "", err
	}
	return cm.Data["config"], nil
}

func (o *k8s) saveSettings(data string) error {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "podreaper-config"},
		Data:       map[string]string{"config": data},
	}
	_, err := o.clientset.CoreV1().ConfigMaps("podreaper").Update(cm)
	if err != nil {
		_, err := o.clientset.CoreV1().ConfigMaps("podreaper").Create(cm)
		return err
	}
	return nil
}
