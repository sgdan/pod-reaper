package main

import (
	"encoding/json"
	"log"
	"sort"
	"time"
)

// Maintain the status JSON that is served to clients
func maintainStatus(s state) {
	now := time.Now().In(&s.timeZone).Format(timeFormat)
	configs := loadConfigs(s)
	configsChanged := false
	states := map[string]nsState{}
	clockTick := time.Tick(5 * time.Second) // trigger clock updates
	cfgTick := time.Tick(60 * time.Second)  // trigger config saves

	for {
		select {
		// send the current status to client
		case s.getStatus <- updateStatus(configs, states, now):

		// update the time displayed in web UI
		case <-clockTick:
			newTime := time.Now().In(&s.timeZone).Format(timeFormat)
			if newTime != now {
				now = newTime
			}

		case state := <-s.updateNsState:
			states[state.Name] = state

		case config := <-s.updateNsConfig:
			configs[config.Name] = config
			configsChanged = true

		// remove namespaces if required
		case ns := <-s.rmNsStatus:
			delete(configs, ns)
			delete(states, ns)

		// send configs to consumer
		case s.getConfigs <- toArray(configs):

		// save configs
		case <-cfgTick:
			if configsChanged {
				err := s.cluster.saveSettings(toArray(configs))
				if err != nil {
					log.Printf("Unable to save configs: %v", err)
				} else {
					configsChanged = false
					log.Printf("Configs saved")
				}
			}
		}

	}
}

func toArray(cfgs map[string]nsConfig) []nsConfig {
	result := []nsConfig{}
	for _, v := range cfgs {
		result = append(result, v)
	}
	return result
}

func loadConfigs(s state) map[string]nsConfig {
	// load existing configs
	configs := map[string]nsConfig{}
	loaded, err := s.cluster.getSettings()
	if err != nil {
		log.Printf("Unable to load configs from cluster: %v", err)
	} else {
		for _, cfg := range loaded {
			configs[cfg.Name] = cfg
		}
	}
	return configs
}

// Update the JSON status to be returned to clients
func updateStatus(configs map[string]nsConfig, states map[string]nsState, clock string) string {
	// create sorted list of keys
	keys := []string{}
	for key := range states {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// add namespaces in sorted key order
	values := []nsStatus{}
	for _, key := range keys {
		cfg, ok := configs[key]
		if !ok {
			cfg = nsConfig{
				Name:  key,
				Limit: 10,
			}
		}
		values = append(values, newStatus(key, states[key], cfg))
	}
	newStatus := status{
		Clock:      clock,
		Namespaces: values,
	}
	newStatusString, _ := json.Marshal(newStatus)
	return string(newStatusString)
}

func newStatus(name string, state nsState, config nsConfig) nsStatus {
	sinceLastStart := time.Now().Unix() - config.LastStarted
	return nsStatus{
		Name:          name,
		HasDownQuota:  state.HasDownQuota,
		CanExtend:     sinceLastStart > 60*60, // running for more than 1hr?
		MemUsed:       state.MemUsed,
		MemLimit:      config.Limit,
		AutoStartHour: config.AutoStartHour,
		Remaining:     state.Remaining,
	}
}
