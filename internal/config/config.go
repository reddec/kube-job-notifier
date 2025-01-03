package config

import (
	"net/http"
	"time"

	"gopkg.in/yaml.v3"
)

type Rule struct {
	Namespace string              `yaml:"namespace,omitempty"`
	Labels    map[string]string   `yaml:"labels,omitempty"`
	Webhooks  []Upstream[Webhook] `yaml:"webhooks,omitempty"`
}

type UpstreamConfig struct {
	Queue    int           `yaml:"queue,omitempty"`
	Retries  int           `yaml:"retries,omitempty"`
	Interval time.Duration `yaml:"interval,omitempty"`
}

type Upstream[T any] struct {
	UpstreamConfig `yaml:",inline"`
	Config         T `yaml:",inline"`
}

func (w *Upstream[T]) UnmarshalYAML(value *yaml.Node) error {
	type alias Upstream[T]
	var def = alias{
		UpstreamConfig: UpstreamConfig{
			Queue:    100,
			Retries:  5,
			Interval: time.Second,
		},
	}

	// weired trick to mix static and dynamic dispatch
	var r any = &def.Config
	if v, ok := (r).(Reset); ok {
		v.Reset()
	}

	if err := value.Decode(&def); err != nil {
		return err
	}
	*w = Upstream[T](def)
	return nil
}

type Reset interface {
	Reset()
}

type Webhook struct {
	URL     *Template            `yaml:"url"`
	Method  string               `yaml:"method,omitempty"`
	Headers map[string]*Template `yaml:"headers,omitempty"`
	Body    *Template            `yaml:"body,omitempty"`
}

func (w *Webhook) Reset() {
	*w = Webhook{
		Method: http.MethodPost,
		Body:   MustTemplate("Job {{.Job.Name}}\n\n{{range .Pods}}{{.Name}}\n\n{{.Logs}}\n\n\n{{end}}"),
	}
}
