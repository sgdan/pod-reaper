package main

type settings struct {
	data map[string]nsConfig
}

func newSettings(data map[string]nsConfig) settings {
	s := settings{data}
	return s
}

func (s settings) getStartHourFor(ns string) *int {
	if config, ok := s.data[ns]; ok {
		return config.AutoStartHour
	}
	return nil
}

func (s settings) getLastStartedFor(ns string) uint64 {
	if config, ok := s.data[ns]; ok {
		return config.LastStarted
	}
	return 0
}
