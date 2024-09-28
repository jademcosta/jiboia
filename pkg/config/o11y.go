package config

const DefaultServiceNameOnO11y = "jiboia"

type O11yConfig struct {
	Tracing TracingConfig `yaml:"tracing"`
	Log     LogConfig     `yaml:"log"`
}

func (o11yConf O11yConfig) fillDefaults() O11yConfig {
	o11yConf.Log = o11yConf.Log.fillDefaults()
	o11yConf.Tracing = o11yConf.Tracing.fillDefaults()
	return o11yConf
}

func (o11yConf O11yConfig) validate() error {
	err := o11yConf.Log.validate()
	if err != nil {
		return err
	}

	return o11yConf.Tracing.validate()
}
