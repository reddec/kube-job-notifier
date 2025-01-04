package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jessevdk/go-flags"
	"github.com/sourcegraph/conc/pool"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/reddec/kube-job-notifier/internal/config"
	"github.com/reddec/kube-job-notifier/internal/engine"
	"github.com/reddec/kube-job-notifier/internal/upstreams"
	"github.com/reddec/kube-job-notifier/internal/upstreams/logger"
	"github.com/reddec/kube-job-notifier/internal/upstreams/webhook"
)

type Config struct {
	Config string `short:"c" long:"config" env:"CONFIG" description:"Path to config file" default:"notify.yaml"`
	// mimic std clientcmd
	KubeConfig string `short:"C" long:"kubeconfig" env:"KUBECONFIG" description:"Path to kubernetes config file to run service outside of cluster"`
	MasterURL  string `long:"master" env:"MASTER_URL" description:"Kuberentes master URL"`
	// general config
	Engine engine.Config `group:"Engine configuration" namespace:"engine" env-namespace:"ENGINE"`
}

func main() {
	var cfg Config
	parser := flags.NewParser(&cfg, flags.Default)
	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		panic(err)
	}
}

func run(cfg Config) error {
	var loader upstreams.Loader

	webhook.Register(&loader)
	logger.Register(&loader)

	rules, err := config.Load(cfg.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	kubeconfig, err := clientcmd.BuildConfigFromFlags(cfg.MasterURL, cfg.KubeConfig)
	if err != nil {
		return fmt.Errorf("get incluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("new clientset: %w", err)
	}

	var wg = pool.New().WithContext(ctx).WithCancelOnError()

	for i, rule := range rules {

		ups, err := loader.Build(rule.UpstreamsConfig)
		if err != nil {
			cancel()
			return fmt.Errorf("loading upstreams: %w", err)
		}

		// start upstreams
		for _, up := range ups {
			slog.Info("starting upstream", "rule_idx", i, "kind", up.Kind())
			wg.Go(up.Run)
		}

		inst := engine.New(cfg.Engine, rule.WatchConfig, ups, clientset)
		slog.Info("started rule", "rule_idx", i)
		wg.Go(inst.Run)
	}

	slog.Info("all rules started", "rules_num", len(rules))

	return wg.Wait()
}
