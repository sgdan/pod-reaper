package main

import (
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

	// example settings
	nineAm := int(9)
	example := map[string]namespaceConfig{
		"ns1": {
			AutoStartHour: nil,
			LastStarted:   0,
		},
		"ns2": {
			AutoStartHour: &nineAm,
			LastStarted:   1589668156345,
		},
	}

	// save settings, then retrieve and check
	k8s.saveSettings(example)
	settings, _ = k8s.getSettings()
	expected, _ := toJSON(example)
	if settings != expected {
		t.Fatalf("Save settings failed\nExpected: %s\nActual: %s", expected, settings)
	}
}

func TestJSON(t *testing.T) {
	example := "{\"default\":{\"autoStartHour\":null,\"lastStarted\":1589668156345},\"ns1\":{\"autoStartHour\":9,\"lastStarted\":0}}"
	converted, _ := fromJSON(example)

	restored, _ := toJSON(converted)
	if example != restored {
		t.Fatalf("JSON conversions failed\nExpected: %s\nActual: %s", example, restored)
	}
}
