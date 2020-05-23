package main

import (
	"log"
	"testing"

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
	k8s.createNamespace("default")
	k8s.getExists("default")
	if !k8s.getExists("default") {
		t.Fatal("default namespace should exist")
	}
}

func TestSettings(t *testing.T) {
	k8s := newTestSimpleK8s()

	// settings should not exist yet
	settings, _ := k8s.getSettings()
	if settings != "" {
		t.Fatal("settings should not exist")
	}

	// create settings
	k8s.saveSettings("some data!")
	settings, err := k8s.getSettings()
	if err != nil {
		log.Printf("Error: %s", err)
	} else {
		log.Printf("Settings: %s", settings)
	}
}
