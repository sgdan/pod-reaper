package main

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestSimpleK8s() *k8s {
	client := k8s{}
	client.clientset = fake.NewSimpleClientset()
	return &client
}

func TestNamespaceExists(t *testing.T) {
	k8s := newTestSimpleK8s()
	if k8s.getExists("default") {
		t.Fatal("default namespace should not exist")
	}
	nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	k8s.clientset.CoreV1().Namespaces().Create(nsSpec)
	k8s.getExists("default")
	if !k8s.getExists("default") {
		t.Fatal("default namespace should exist")
	}
}
