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

func TestNamespaces(t *testing.T) {
	k8s := newTestSimpleK8s()
	namespaces, _ := k8s.getNamespaces()
	if len(namespaces) != 0 {
		t.Fatal("should not be any namespaces")
	}
	k8s.createNamespace("one")
	k8s.createNamespace("two")
	namespaces, _ = k8s.getNamespaces()
	if !contains(namespaces, "one") || !contains(namespaces, "two") {
		t.Fatal("namespaces 'one' and 'two' should be included")
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

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Can't test deletePods because fake client doesn't support DeleteCollection
// func TestPods(t *testing.T) {
// 	k8s := newTestSimpleK8s()
// 	defPods := k8s.clientset.CoreV1().Pods("default")

// 	// start with no pods
// 	pods, _ := defPods.List(metav1.ListOptions{})
// 	if len(pods.Items) != 0 {
// 		t.Fatalf("Should be no pods initially")
// 	}

// 	// create some pods
// 	names := [3]string{"p1", "p2", "p3"}
// 	for _, name := range names {
// 		pod := &core.Pod{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name: name,
// 			},
// 			Spec: core.PodSpec{
// 				Containers: []core.Container{
// 					{
// 						Name:  "nginx",
// 						Image: "nginx",
// 					},
// 				},
// 			},
// 		}
// 		defPods.Create(pod)
// 	}
// 	pods, _ = defPods.List(metav1.ListOptions{})
// 	if len(pods.Items) != 3 {
// 		t.Fatalf("Should be 3 pods")
// 	}

// 	defPods.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{})
// 	pods, _ = defPods.List(metav1.ListOptions{})
// 	n := len(pods.Items)
// 	log.Printf("length: %x", n)
// }
