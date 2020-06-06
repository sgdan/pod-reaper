package main

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
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
	if len(namespaces) != 2 {
		t.Fatalf("should be 2 namespaces not %v", len(namespaces))
	}
	if !contains(namespaces, "one") || !contains(namespaces, "two") {
		t.Fatal("namespaces 'one' and 'two' should be included")
	}
}

func TestSettings(t *testing.T) {
	k8s := newTestSimpleK8s()

	// settings should not exist yet
	_, err := k8s.getSettings()
	if err == nil {
		t.Fatal("settings should not exist")
	}

	// example settings
	nineAm := int(9)
	example := []nsConfig{
		{
			Name:          "ns1",
			AutoStartHour: nil,
			LastStarted:   0,
		},
		{
			Name:          "ns2",
			AutoStartHour: &nineAm,
			LastStarted:   1589668156345,
		},
	}

	// save settings, then retrieve and check
	k8s.saveSettings(example)
	settings, _ := k8s.getSettings()
	if !reflect.DeepEqual(settings, example) {
		t.Fatalf("Save settings failed\nExpected: %v\nActual: %v", example, settings)
	}
}

func TestJSON(t *testing.T) {
	example := "[{\"name\":\"default\",\"autoStartHour\":null,\"lastStarted\":1589668156345},{\"name\":\"ns1\",\"autoStartHour\":9,\"lastStarted\":0}]"
	converted, _ := fromJSON(example)

	restored, _ := toJSON(converted)
	if example != restored {
		t.Fatalf("JSON conversions failed\nExpected: %s\nActual: %s", example, restored)
	}
}

func TestResourceQuotas(t *testing.T) {
	q2 := resource.Quantity{Format: "2Gi"}
	q5 := resource.Quantity{Format: "5Gi"}

	// should be no resource quota initially
	k8s := newTestSimpleK8s()
	rq, _ := k8s.getResourceQuota("default", "testrq")
	if rq != nil {
		t.Fatalf("Not expecting to find resource quota: %v", rq)
	}

	// create
	_, err := k8s.setResourceQuota("default", "testrq", q2)
	if err != nil {
		t.Fatalf("Should be able to create resource quota: %v", err)
	}
	rq, err = k8s.getResourceQuota("default", "testrq")
	if err != nil {
		t.Fatalf("Should be able to get resource quota: %v", err)
	}
	limit := rq.Spec.Hard.Memory().Format
	if "2Gi" != limit {
		t.Fatalf("Expected 2Gi limit but was %v", limit)
	}

	// update
	_, err = k8s.setResourceQuota("default", "testrq", q5)
	if err != nil {
		t.Fatalf("Should be able to update resource quota: %v", err)
	}
	rq, err = k8s.getResourceQuota("default", "testrq")
	if err != nil {
		t.Fatalf("Should be able to get resource quota: %v", err)
	}
	limit = rq.Spec.Hard.Memory().Format
	if "5Gi" != limit {
		t.Fatalf("Expected 5Gi limit but was %v", limit)
	}

	// delete
	err = k8s.removeResourceQuota("default", "testrq")
	if err != nil {
		t.Fatalf("Should be able to delete resource quota: %v", err)
	}
	exists := k8s.hasResourceQuota("default", "testrq")
	if exists {
		t.Fatalf("Resource quota should no longer exist")
	}
}

/*
// Can't test deletePods because fake client doesn't support DeleteCollection
func TestPods(t *testing.T) {
	k8s := newTestSimpleK8s()
	defPods := k8s.clientset.CoreV1().Pods("default")

	// start with no pods
	pods, _ := defPods.List(metav1.ListOptions{})
	if len(pods.Items) != 0 {
		t.Fatalf("Should be no pods initially")
	}

	// create some pods
	names := [3]string{"p1", "p2", "p3"}
	for _, name := range names {
		pod := &core.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: core.PodSpec{
				Containers: []core.Container{
					{
						Name:  "nginx",
						Image: "nginx",
					},
				},
			},
		}
		defPods.Create(pod)
	}
	pods, _ = defPods.List(metav1.ListOptions{})
	if len(pods.Items) != 3 {
		t.Fatalf("Should be 3 pods")
	}

	defPods.DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{})
	pods, _ = defPods.List(metav1.ListOptions{})
	n := len(pods.Items)
	log.Printf("length: %x", n)
}
*/

func TestRemaining(t *testing.T) {
	start := time.Now().Unix() // unix time in seconds (int64)
	m := int64(60)             // seconds in minute
	stop := start + 8*60*m     // 8 hrs after start

	check("", rem(0, stop), t)
	check("", rem(stop-m+1, start), t)
	check("1m", rem(start, stop-m), t)
	check("5m", rem(start, stop-5*m), t)
	check("10m", rem(start, stop-10*m), t)
	check("1h 03m", rem(start, stop-63*m), t)
	check("7h 59m", rem(start, start+m), t)
	check("7h 59m", rem(start, start+1), t)
	check("", rem(start, start), t)
	check("", rem(start, start-20*m), t)
}

func rem(start int64, stop int64) string {
	return remaining(remainingSeconds(start, stop))
}

func check(expected string, actual string, t *testing.T) {
	if expected != actual {
		t.Fatalf("Expected '%s' but was '%s'", expected, actual)
	}
}

func checkInt(expected int, actual int, t *testing.T) {
	if expected != actual {
		t.Fatalf("Expected %v but was %v", expected, actual)
	}
}

func TestAutoStart(t *testing.T) {
	wed8pm := toTime("2019-11-13T20:00:00Z", t)
	wedAfter8pm := toTime("2019-11-13T20:32:00Z", t)

	// something is always more recent than nil
	check("0001-01-01T00:00:00Z", toString(mostRecent(nil, time.Time{})), t)

	// wednesday is more recent than beginning of time
	started := mostRecent(&wed8pm, time.Time{})
	check("2019-11-13T20:00:00Z", toString(started), t)

	// check lastScheduled
	hour := 20
	lsched := lastScheduled(&hour, wedAfter8pm)
	lschedString := toString(time.Unix(lsched, 0).In(time.UTC))
	check("2019-11-13T20:00:00Z", lschedString, t)
	hour = 17
	lsched = lastScheduled(&hour, wedAfter8pm)
	lschedString = toString(time.Unix(lsched, 0).In(time.UTC))
	check("2019-11-13T17:00:00Z", lschedString, t)

	// check hoursFrom
	checkInt(0, hoursFrom(started, wedAfter8pm), t)
	checkInt(3, hoursFrom(time.Unix(lsched, 0), wedAfter8pm), t)
}

func toTime(value string, t *testing.T) time.Time {
	result, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("Unable to parse time: %v", value)
	}
	return result
}

func toString(value time.Time) string {
	return value.Format(time.RFC3339)
}
