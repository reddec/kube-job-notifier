package config

import (
	"time"

	"gopkg.in/yaml.v3"
)

type Rule struct {
	WatchConfig     `yaml:",inline"`
	UpstreamsConfig map[string][]Upstream `yaml:",inline"`
}

type WatchConfig struct {
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type Upstream struct {
	Queue    int           `yaml:"queue,omitempty"`
	Retries  int           `yaml:"retries,omitempty"`
	Interval time.Duration `yaml:"interval,omitempty"`
	Config   *yaml.Node    `yaml:"-"` // rest is for upstream specific config
}

func (w *Upstream) UnmarshalYAML(value *yaml.Node) error {
	type alias Upstream
	var def = alias{
		Queue:    100,
		Retries:  5,
		Interval: time.Second,
	}

	if err := value.Decode(&def); err != nil {
		return err
	}
	def.Config = value
	*w = Upstream(def)
	return nil
}
