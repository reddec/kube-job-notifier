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

type ComplexTemplate RenderFunc

func (js *ComplexTemplate) UnmarshalYAML(value *yaml.Node) error {
	var val any
	if err := value.Decode(&val); err != nil {
		return err
	}
	rf, err := NewComplexTemplate(val)
	if err != nil {
		return err
	}
	*js = ComplexTemplate(rf)
	return nil
}

type RenderFunc func(rc *RenderContext) (any, error)

func MustComplexTemplate(content any) ComplexTemplate {
	v, err := NewComplexTemplate(content)
	if err != nil {
		panic(err)
	}
	return v
}

func NewComplexTemplate(value any) (ComplexTemplate, error) {
	switch v := value.(type) {
	case []byte:
		t, err := NewTemplate(string(v))
		if err != nil {
			return nil, err
		}
		return func(rc *RenderContext) (any, error) {
			return t.Render(rc)
		}, nil
	case string:
		t, err := NewTemplate(v)
		if err != nil {
			return nil, err
		}
		return func(rc *RenderContext) (any, error) {
			return t.Render(rc)
		}, nil
	case []any:
		var ans []ComplexTemplate
		for i, item := range v {
			rf, err := NewComplexTemplate(item)
			if err != nil {
				return nil, fmt.Errorf("parse item %d: %w", i, err)
			}
			ans = append(ans, rf)
		}

		return func(rc *RenderContext) (any, error) {
			var out = make([]any, len(ans))
			for i, rf := range ans {
				res, err := rf(rc)
				if err != nil {
					return nil, fmt.Errorf("render item %d: %w", i, err)
				}
				out[i] = res
			}
			return out, nil
		}, nil

	case map[string]any:
		var out = make(map[string]ComplexTemplate)
		for k, val := range v {
			rf, err := NewComplexTemplate(val)
			if err != nil {
				return nil, fmt.Errorf("parse field %q: %w", k, err)
			}
			out[k] = rf
		}

		return func(rc *RenderContext) (any, error) {
			var res = make(map[string]any)
			for k, rf := range out {
				field, err := rf(rc)
				if err != nil {
					return nil, fmt.Errorf("render field %q: %w", k, err)
				}
				res[k] = field
			}
			return res, nil
		}, nil
	default:
		return func(rc *RenderContext) (any, error) {
			return v, nil
		}, nil
	}
}
