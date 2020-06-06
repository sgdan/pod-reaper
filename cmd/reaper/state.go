package main

import (
	"time"
)

type state struct {
	timeZone          time.Location
	ignoredNamespaces []string
	cluster           k8s // access to the cluster

	// changes and updates
	updateNs       chan nsStatus // signal namespace updated
	triggerNs      chan string   // signal that namespace needs to be updated
	rmNs           chan string   // signal namespace removed
	updateNsConfig chan nsConfig // signal namepsace config updated

	// getting data
	getStatus     chan string              // get the current status JSON
	getNamespaces chan map[string]nsStatus // get snapshot of current namespaces
	getConfigs    chan []nsConfig          // get the current namespace configs
}

func newState(tz time.Location, ignored []string, cluster k8s) state {
	s := state{
		timeZone:          tz,
		ignoredNamespaces: ignored,
		cluster:           cluster,
		updateNs:          make(chan nsStatus),
		triggerNs:         make(chan string),
		rmNs:              make(chan string),
		updateNsConfig:    make(chan nsConfig),
		getStatus:         make(chan string),
		getNamespaces:     make(chan map[string]nsStatus),
		getConfigs:        make(chan []nsConfig),
	}
	return s
}

// Get the names of the namespaces currently monitored
func (s state) existingNamespaces() []string {
	statuses := <-s.getNamespaces
	result := []string{}
	for key := range statuses {
		result = append(result, key)
	}
	return result
}

func (s state) getConfigFor(ns string) nsConfig {
	configs := <-s.getConfigs
	for _, cfg := range configs {
		if cfg.Name == ns {
			return cfg
		}
	}
	return nsConfig{} // return an empty one if existing not found
}
