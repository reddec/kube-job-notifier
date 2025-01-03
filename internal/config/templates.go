package config

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/batch/v1"
	v2 "k8s.io/api/core/v1"
)

type Pod struct {
	*v2.Pod
	Logs string
}

// Log is alias to logs to have better UX.
func (p *Pod) Log() string {
	return p.Logs
}

type RenderContext struct {
	Job  *v1.Job
	Pods []Pod
}

func MustTemplate(value string) *Template {
	v, err := NewTemplate(value)
	if err != nil {
		panic(err)
	}
	return v
}

func NewTemplate(content string) (*Template, error) {
	tmpl, err := template.New("").Funcs(sprig.FuncMap()).Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	return &Template{
		tpl: tmpl,
	}, nil
}

type Template struct {
	tpl *template.Template
}

func (t *Template) UnmarshalYAML(value *yaml.Node) error {
	var content string
	if err := value.Decode(&content); err != nil {
		return err
	}

	v, err := NewTemplate(content)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	*t = *v
	return nil
}

func (t *Template) Render(rc *RenderContext) (string, error) {
	var buf bytes.Buffer
	err := t.tpl.Execute(&buf, rc)
	return buf.String(), err
}
