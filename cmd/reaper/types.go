package main

import "time"

/*
Specification contains default configuration for this app
that can be overridden using environment variables.
See https://github.com/sethvargo/go-envconfig
*/
type Specification struct {
	IgnoredNamespaces []string `env:"IGNORED_NAMESPACES,default=kube-system,kube-public,kube-node-lease,podreaper,docker"`
	ZoneID            string   `env:"ZONE_ID,default=UTC"`
	CorsEnabled       bool     `env:"CORS_ENABLED,default=false"`
	CorsOrigins       []string `env:"CORS_ORIGINS,default=http://localhost:3000"`
	InCluster         bool     `env:"IN_CLUSTER,default=false"`
	StaticFiles       string   `env:"STATIC_FILES,default="`

	// timings
	NamespaceTick  time.Duration `env:"NAMESPACE_TICK,default=11s"`
	NamespacesTick time.Duration `env:"NAMESPACES_TICK,default=17s"`
	RangerTick     time.Duration `env:"RANGER_TICK,default=41s"`
	ClockTick      time.Duration `env:"CLOCK_TICK,default=13s"`
	ConfigTick     time.Duration `env:"CONFIG_TICK,default=17s"`
	ReaperTick     time.Duration `env:"REAPER_TICK,default=29s"`
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
