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
	CorsEnabled       bool     `default:"true" envconfig:"cors_enabled"`
	CorsOrigins       []string `default:"http://localhost:3000" envconfig:"cors_origins"`
	InCluster         bool     `default:"false" envconfig:"in_cluster"`
	StaticFiles       string   `default:"" envconfig:"static_files"`
}

// This is the status displayed by the UI
type status struct {
	Clock      string     `json:"clock"`
	Namespaces []nsStatus `json:"namespaces"`
}

// Namespace settings configured via the UI
type nsConfig struct {
	Name          string `json:"name"`
	AutoStartHour *int   `json:"autoStartHour"`
	LastStarted   int64  `json:"lastStarted"`
	Limit         int    `json:"limit"`
}

// Namespace data used in backend
type nsState struct {
	Name          string
	HasDownQuota  bool
	MemUsed       int
	Remaining     string
	LastScheduled int64
}

// Namespace data required by UI
type nsStatus struct {
	Name          string `json:"name"`
	HasDownQuota  bool   `json:"hasDownQuota"`
	CanExtend     bool   `json:"canExtend"`
	MemUsed       int    `json:"memUsed"`
	MemLimit      int    `json:"memLimit"`
	AutoStartHour *int   `json:"autoStartHour"`
	Remaining     string `json:"remaining"`
}

// POST requests from UI
type startRequest struct {
	Namespace string `json:"namespace"`
	StartHour *int   `json:"startHour"`
}
type limitRequest struct {
	Namespace string `json:"namespace"`
	Limit     int    `json:"limit"`
}
