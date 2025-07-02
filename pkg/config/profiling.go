package config

type ProfilingConfig struct {
	Enabled bool `yaml:"enable_pprof"`
}

func (profileConf ProfilingConfig) fillDefaults() ProfilingConfig {
	return profileConf
}

func (profileConf ProfilingConfig) validate() error {
	return nil
}
