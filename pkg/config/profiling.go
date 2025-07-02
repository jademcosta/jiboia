package config

type ProfilingConfig struct {
	Enabled bool `yaml:"enabled_pprof"`
}

func (profileConf ProfilingConfig) fillDefaults() ProfilingConfig {
	return profileConf
}

func (profileConf ProfilingConfig) validate() error {
	return nil
}
