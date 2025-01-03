package webhook

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/reddec/kube-job-notifier/internal/config"
)

func New(cfg config.Webhook) *Webhook {
	return &Webhook{
		config: cfg,
	}
}

type Webhook struct {
	config config.Webhook
}

func (w Webhook) Send(ctx context.Context, info *config.RenderContext) error {
	const peekReply = 1024
	url, err := w.config.URL.Render(info)
	if err != nil {
		return fmt.Errorf("render url: %w", err)
	}

	payload, err := w.config.Body.Render(info)
	if err != nil {
		return fmt.Errorf("render body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, w.config.Method, url, strings.NewReader(payload))
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
