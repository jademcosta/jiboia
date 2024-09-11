package config

type O11yConfig struct {
	TracingEnabled bool      `yaml:"tracing_enabled"`
	Log            LogConfig `yaml:"log"`
}

func (o11yConf O11yConfig) fillDefaults() O11yConfig {
	o11yConf.Log = o11yConf.Log.fillDefaults()
	return o11yConf
}

func (o11yConf O11yConfig) validate() error {
	return o11yConf.Log.validate()
}
