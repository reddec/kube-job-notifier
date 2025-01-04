package logger

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/reddec/kube-job-notifier/internal/config"
	"github.com/reddec/kube-job-notifier/internal/upstreams"
)

type Config struct {
	Message *config.Template `yaml:"message"`
}

func Default() Config {
	return Config{
		Message: config.MustTemplate("Job {{.Job.Name}}\n\n{{range .Pods}}{{.Name}}\n\n{{.Logs}}\n\n\n{{end}}"),
	}
}

func New(cfg Config) *Logger {
	return &Logger{
		cfg: cfg,
	}
}

type Logger struct {
	cfg Config
}

func (lg *Logger) Send(ctx context.Context, info *config.RenderContext) error {
	txt, err := lg.cfg.Message.Render(info)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	slog.Info(txt)
	return nil
}

func Register(loader *upstreams.Loader) {
	upstreams.Register[Config](loader, "logger", Default, New)
}
