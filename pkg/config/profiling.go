package config

type ProfilingConfig struct {
	Enabled bool `json:"enabled_pprof" yaml:"enabled_pprof"`
}

func (profileConf ProfilingConfig) fillDefaults() ProfilingConfig {
	return profileConf
}

func (profileConf ProfilingConfig) validate() error {
	return nil
}
