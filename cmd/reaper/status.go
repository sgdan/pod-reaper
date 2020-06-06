package main

import (
	"encoding/json"
	"log"
	"sort"
	"time"
)

// Maintain the status JSON that is served to clients
func maintainStatus(s state) {
	emptyStatusString, _ := json.Marshal(status{})
	status := string(emptyStatusString)
	now := time.Now().In(&s.timeZone).Format(timeFormat)
	namespaces := make(map[string]nsStatus)
	tick := time.Tick(5 * time.Second) // trigger clock updates

	for {
		select {
		// send the current status to client
		case s.getStatus <- status:

		// update the time displayed in web UI
		case <-tick:
			newTime := time.Now().In(&s.timeZone).Format(timeFormat)
			if newTime != now {
				now = newTime
				status = updateStatus(namespaces, now)
				log.Printf("Updated time to: %v", now)
			}

		// update current status when a namespace changes
		case update := <-s.updateNs:
			namespaces[update.Name] = update
			status = updateStatus(namespaces, now)
			log.Printf("Namespace updated: %v", update)

		// remove namespaces if required
		case ns := <-s.rmNsStatus:
			log.Printf("Removing namespace %v", ns)
			delete(namespaces, ns)
			status = updateStatus(namespaces, now)
			// case valid := <-nsNames:
			// 	for key := range nsStatuses {
			// 		if !contains(valid, key) {
			// 			log.Printf("Removing namespace %v", key)
			// 			delete(nsStatuses, key)
			// 		}
			// 	}
			// }
		}

	}
}

// Update the JSON status to be returned to clients
func updateStatus(statuses map[string]nsStatus, clock string) string {
	// create sorted list of keys
	keys := make([]string, len(statuses))
	i := 0
	for key := range statuses {
		keys[i] = key
		i++
	}
	sort.Strings(keys)

	// add namespaces in sorted key order
	values := make([]nsStatus, len(statuses))
	for index, key := range keys {
		values[index] = statuses[key]
	}
	newStatus := status{
		Clock:      clock,
		Namespaces: values,
	}
	newStatusString, _ := json.Marshal(newStatus)
	return string(newStatusString)
}
