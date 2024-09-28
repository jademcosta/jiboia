package config

type TracingConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ServiceName string `yaml:"service_name"`
}

func (tracingConf TracingConfig) fillDefaults() TracingConfig {
	if tracingConf.ServiceName == "" {
		tracingConf.ServiceName = DefaultServiceNameOnO11y
	}

	return tracingConf
}

func (tracingConf TracingConfig) validate() error {
	return nil
}
