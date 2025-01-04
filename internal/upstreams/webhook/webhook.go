package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/reddec/kube-job-notifier/internal/config"
	"github.com/reddec/kube-job-notifier/internal/upstreams"
)

type Config struct {
	URL     *config.Template            `yaml:"url"`
	Method  string                      `yaml:"method,omitempty"`
	Headers map[string]*config.Template `yaml:"headers,omitempty"`
	Body    config.ComplexTemplate      `yaml:"body,omitempty"`
}

func Default() Config {
	return Config{
		Method: http.MethodPost,
		Body:   config.MustComplexTemplate("Job {{.Job.Name}}\n\n{{range .Pods}}{{.Name}}\n\n{{.Logs}}\n\n\n{{end}}"),
	}
}

func New(cfg Config) *Webhook {
	return &Webhook{
		config: cfg,
	}
}

type Webhook struct {
	config Config
}

func (w Webhook) Send(ctx context.Context, info *config.RenderContext) error {
	const peekReply = 1024
	url, err := w.config.URL.Render(info)
	if err != nil {
		return fmt.Errorf("render url: %w", err)
	}

	payload, err := w.config.Body(info)
	if err != nil {
		return fmt.Errorf("render body: %w", err)
	}

	var isJSON bool
	var reader io.Reader
	if v, ok := payload.(string); ok {
		reader = strings.NewReader(v)
	} else if v, ok := payload.(io.Reader); ok {
		reader = v
	} else if v, ok := payload.([]byte); ok {
		reader = bytes.NewReader(v)
	} else {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		isJSON = true
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, w.config.Method, url, reader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	for k, v := range w.config.Headers {
		content, err := v.Render(info)
		if err != nil {
			return fmt.Errorf("render header %q: %w", k, err)
		}
		req.Header.Set(k, content)
	}

	if isJSON && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 2 {
		return nil
	}
	errContent, _ := io.ReadAll(io.LimitReader(resp.Body, peekReply))

	return fmt.Errorf("send request: status %d, body: %s", resp.StatusCode, string(errContent))
}

func Register(loader *upstreams.Loader) {
	upstreams.Register[Config](loader, "webhook", Default, New)
}
