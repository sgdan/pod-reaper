package main

import (
	"time"
)

type state struct {
	timeZone          time.Location
	ignoredNamespaces []string
	cluster           k8s // access to the cluster

	// changes and updates
	triggerNs      chan string   // signal that namespace needs to be updated
	updateNsState  chan nsState  // signal namespace updated
	updateNsConfig chan nsConfig // signal namepsace config updated

	// signal namespace removal
	rmNsConfig chan string
	rmNsStatus chan string

	// getting data
	getStatus  chan string     // get the current status JSON
	getConfigs chan []nsConfig // get the current namespace configs
	getStates  chan []nsState  // get cached namespace state
}

func newState(tz time.Location, ignored []string, cluster k8s) state {
	s := state{
		timeZone:          tz,
		ignoredNamespaces: ignored,
		cluster:           cluster,
		triggerNs:         make(chan string),
		rmNsConfig:        make(chan string),
		rmNsStatus:        make(chan string),
		updateNsState:     make(chan nsState),
		updateNsConfig:    make(chan nsConfig),
		getStatus:         make(chan string),
		getConfigs:        make(chan []nsConfig),
		getStates:         make(chan []nsState),
	}
	return s
}

func (s state) getConfigFor(ns string) nsConfig {
	configs := <-s.getConfigs
	for _, cfg := range configs {
		if cfg.Name == ns {
			return cfg
		}
	}
	// return empty one if existing not found
	return nsConfig{Name: ns}
}
