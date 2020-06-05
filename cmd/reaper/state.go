package main

import (
	"time"
)

type state struct {
	TimeZone          time.Location
	IgnoredNamespaces []string

	// changes and updates
	updateNs       chan nsStatus // signal namespace updated
	rmNs           chan string   // signal namespace removed
	updateNsConfig chan nsConfig // signal namepsace config updated
	rmNsConfig     chan string   // signal namespace config removed

	// getting data
	getStatus  chan string     // get the current status JSON
	getConfigs chan []nsConfig // get the current namespace configs
}

func newState(timeZone time.Location, ignoredNamespaces []string) state {
	s := state{
		TimeZone:          timeZone,
		IgnoredNamespaces: ignoredNamespaces,
		updateNs:          make(chan nsStatus),
		rmNs:              make(chan string),
		updateNsConfig:    make(chan nsConfig),
		rmNsConfig:        make(chan string),
		getStatus:         make(chan string),
		getConfigs:        make(chan []nsConfig),
	}
	return s
}
