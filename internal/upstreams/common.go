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

func NewAsync(cfg config.UpstreamConfig, next Upstream) *AsyncUpstream {
	return &AsyncUpstream{
		queue: make(chan *config.RenderContext, cfg.Queue),
		next:  next,
		cfg:   cfg,
	}
}

type AsyncUpstream struct {
	queue chan *config.RenderContext
	cfg   config.UpstreamConfig
	next  Upstream
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

func (au *AsyncUpstream) sendMessage(ctx context.Context, info *config.RenderContext) error {
	for i := 0; i <= au.cfg.Retries; i++ {
		err := au.next.Send(ctx, info)
		if err == nil {
			return nil
		}
		slog.Error("attempt failed to send message", "attempt", i+1, "retries", au.cfg.Retries, "error", err)
		if i < au.cfg.Retries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(au.cfg.Interval):
			}
		}
	}
	return fmt.Errorf("all attempts failed")
}
