package main

/*
Specification contains default configuration for this app
that can be overridden using environment variables.
See https://github.com/kelseyhightower/envconfig

Overriding env values for backwards compatibility with the
previous Kotlin/Micronaut code.
*/
type Specification struct {
	IgnoredNamespaces []string `default:"kube-system,kube-public,kube-node-lease,podreaper,docker" envconfig:"ignored_namespaces"`
	ZoneID            string   `default:"UTC" envconfig:"zone_id"`
}

type nsConfig struct {
	Name          string `json:"name"`
	AutoStartHour *int   `json:"autoStartHour"`
	LastStarted   uint64 `json:"lastStarted"`
}

type status struct {
	Clock      string     `json:"clock"`
	Namespaces []nsStatus `json:"namespaces"`
}

type nsStatus struct {
	// used by UI frontend
	Name         string `json:"name"`
	HasDownQuota bool   `json:"hasDownQuota"`
	CanExtend    bool   `json:"canExtend"`
	MemUsed      int    `json:"memUsed"`
	MemLimit     int    `json:"memLimit"`
	// AutoStartHour *int   `json:"autoStartHour"`
	// Remaining     string `json:"remaining"`

	// backend only
	// hasResourceQuota bool
	// lastScheduled ???
	// lastStarted uint64
}
