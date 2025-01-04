package upstreams

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/reddec/kube-job-notifier/internal/config"
)

type Upstream interface {
	Send(ctx context.Context, info *config.RenderContext) error
}

type AsyncUpstream struct {
	queue    chan *config.RenderContext
	retries  int
	interval time.Duration
	kind     string
	next     Upstream
}

func (au *AsyncUpstream) Send(ctx context.Context, info *config.RenderContext) error {
	select {
	case au.queue <- info:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("upstream queue is full")
	}
}

func (au *AsyncUpstream) Run(ctx context.Context) error {
	for {
		select {
		case info := <-au.queue:
			if err := au.sendMessage(ctx, info); err != nil {
				slog.Error("failed to send message", "err", err)
			} else {
				slog.Info("message delivered")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (au *AsyncUpstream) Kind() string {
	return au.kind
}

func (au *AsyncUpstream) sendMessage(ctx context.Context, info *config.RenderContext) error {
	for i := 0; i <= au.retries; i++ {
		err := au.next.Send(ctx, info)
		if err == nil {
			return nil
		}
		slog.Error("attempt failed to send message", "attempt", i+1, "retries", au.retries, "error", err)
		if i < au.retries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(au.interval):
			}
		}
	}
	return fmt.Errorf("all attempts failed")
}
