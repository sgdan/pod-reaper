package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

func (o *k8s) createNamespace(name string) {
	nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	o.clientset.CoreV1().Namespaces().Create(context.Background(), nsSpec, metav1.CreateOptions{})
}

func (o *k8s) getExists(namespace string) bool {
	_, err := o.clientset.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	return err == nil
}

func (o *k8s) getStatusOf(namespace string) (string, error) {
	ns, err := o.clientset.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(ns.Status.Phase), nil
}

func (o *k8s) getConfigMap(name string) (*v1.ConfigMap, error) {
	return o.clientset.CoreV1().ConfigMaps("podreaper").
		Get(context.Background(), name, metav1.GetOptions{})
}

func (o *k8s) getSettings() ([]nsConfig, error) {
	cm, err := o.getConfigMap(configMapName)
	if err != nil {
		return nil, err
	}
	return fromJSON(cm.Data["config"])
}

func (o *k8s) saveSettings(data []nsConfig) error {
	jsonData, err := toJSON(data)
	if err != nil {
		return fmt.Errorf("Unable to convert settings to JSON: %v", err)
	}
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: configMapName},
		Data:       map[string]string{"config": jsonData},
	}
	cms := o.clientset.CoreV1().ConfigMaps("podreaper")
	_, err = cms.Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		_, err := cms.Create(context.Background(), cm, metav1.CreateOptions{})
		return err
	}
	return nil
}

func toJSON(settings []nsConfig) (string, error) {
	result, err := json.Marshal(settings)
	return string(result), err
}

func fromJSON(data string) ([]nsConfig, error) {
	result := []nsConfig{}
	err := json.Unmarshal([]byte(data), &result)
	return result, err
}

func (o *k8s) deletePods(namespace string) error {
	return o.clientset.CoreV1().Pods(namespace).DeleteCollection(context.Background(),
		metav1.DeleteOptions{}, metav1.ListOptions{})
}

func (o *k8s) getNamespaces() ([]string, error) {
	nsList, err := o.clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Unable to list namepsaces: %v", err)
	}
	items := nsList.Items
	result := make([]string, len(items))
	for i, next := range items {
		result[i] = next.ObjectMeta.Name
	}
	return result, nil
}

func (o *k8s) getResourceQuota(ns string, rqName string) (*v1.ResourceQuota, error) {
	return o.clientset.CoreV1().ResourceQuotas(ns).Get(context.Background(), rqName, metav1.GetOptions{})
}

func (o *k8s) hasResourceQuota(ns string, rqName string) bool {
	_, err := o.getResourceQuota(ns, rqName)
	return err == nil
}

func (o *k8s) setResourceQuota(ns string, rqName string, limit resource.Quantity) (*v1.ResourceQuota, error) {
	rq := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: rqName},
		Spec: v1.ResourceQuotaSpec{
			Hard: v1.ResourceList{
				v1.ResourceMemory: limit,
			},
		},
	}
	rqs := o.clientset.CoreV1().ResourceQuotas(ns)
	if o.hasResourceQuota(ns, rqName) {
		return rqs.Update(context.Background(), rq, metav1.UpdateOptions{})
	}
	return rqs.Create(context.Background(), rq, metav1.CreateOptions{})
}

func (o *k8s) removeResourceQuota(ns string, rqName string) error {
	return o.clientset.CoreV1().ResourceQuotas(ns).Delete(context.Background(), rqName, metav1.DeleteOptions{})
}

// Create default limit range for namespace if it doesn't exist
func (o *k8s) checkLimitRange(ns string) {
	status, err := o.getStatusOf(ns)
	if err != nil {
		log.Printf("Ignoring limit range for %v because it has no status", ns)
		return
	}
	if status != "Active" {
		log.Printf("Ignoring limit range for %v because it has status %v", ns, status)
		return
	}

	lr := &v1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: limitRangeName},
		Spec: v1.LimitRangeSpec{
			Limits: []v1.LimitRangeItem{{
				Type:           v1.LimitTypeContainer,
				DefaultRequest: v1.ResourceList{v1.ResourceMemory: resource.MustParse(podRequest)},
				Default:        v1.ResourceList{v1.ResourceMemory: resource.MustParse(podLimit)},
			}},
		},
	}
	lrs := o.clientset.CoreV1().LimitRanges(ns)
	_, err = lrs.Get(context.Background(), limitRangeName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Creating default limit range for %v", ns)
		lrs.Create(context.Background(), lr, metav1.CreateOptions{})
	}
}
