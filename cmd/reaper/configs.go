package main

import "log"

// Manage the namespace configurations stored in k8s config map
func maintainConfigs(s state) {
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

	for {
		select {
		// send configs to consumer
		case s.getConfigs <- toArray(configs):

		case ns := <-s.rmNsConfig:
			delete(configs, ns)

		// update a config and store to k8s config map
		case update := <-s.updateNsConfig:
			configs[update.Name] = update
			err = s.cluster.saveSettings(toArray(configs))
			if err != nil {
				log.Printf("Unable to save configs to cluster: %v", err)
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
