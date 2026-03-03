package config

const DefaultServiceNameOnO11y = "jiboia"

type O11yConfig struct {
	Tracing           TracingConfig   `json:"tracing"             yaml:"tracing"`
	Log               LogConfig       `json:"log"                 yaml:"log"`
	Profiling         ProfilingConfig `json:"profiling"           yaml:"profiling"`
	ConfigDumpEnabled bool            `json:"config_dump_enabled" yaml:"config_dump_enabled"`
}

func (o11yConf O11yConfig) fillDefaults() O11yConfig {
	o11yConf.Log = o11yConf.Log.fillDefaults()
	o11yConf.Tracing = o11yConf.Tracing.fillDefaults()
	o11yConf.Profiling = o11yConf.Profiling.fillDefaults()
	return o11yConf
}

func (o11yConf O11yConfig) validate() error {
	err := o11yConf.Log.validate()
	if err != nil {
		return err
	}

	err = o11yConf.Profiling.validate()
	if err != nil {
		return err
	}

	return o11yConf.Tracing.validate()
}
