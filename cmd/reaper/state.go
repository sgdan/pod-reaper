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
	updateNsConfig chan nsConfig // signal namepsace config updated

	// signal namespace removal
	rmNsConfig chan string
	rmNsStatus chan string

	// getting data
	getStatus  chan string     // get the current status JSON
	getConfigs chan []nsConfig // get the current namespace configs
}

func newState(tz time.Location, ignored []string, cluster k8s) state {
	s := state{
		timeZone:          tz,
		ignoredNamespaces: ignored,
		cluster:           cluster,
		updateNs:          make(chan nsStatus),
		triggerNs:         make(chan string),
		rmNsConfig:        make(chan string),
		rmNsStatus:        make(chan string),
		updateNsConfig:    make(chan nsConfig),
		getStatus:         make(chan string),
		getConfigs:        make(chan []nsConfig),
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
	return nsConfig{} // return an empty one if existing not found
}
