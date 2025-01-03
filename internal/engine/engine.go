package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/sourcegraph/conc/pool"
	v2 "k8s.io/api/batch/v1"
	v3 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/reddec/kube-job-notifier/internal/config"
	"github.com/reddec/kube-job-notifier/internal/upstreams"
	"github.com/reddec/kube-job-notifier/internal/upstreams/webhook"
)

type Config struct {
	Tail        int64 `long:"tail" env:"TAIL" description:"Limit number of last lines to include" default:"20"`
	LogsBytes   int64 `long:"logs-bytes" env:"LOGS_BYTES" description:"Limit number of bytes in logs to protect service" default:"65535"`
	DedupCache  int   `long:"dedup-cache" env:"DEDUP_CACHE" description:"Cache size for events deduplication" default:"8192"`
	SkipPreload bool  `long:"skip-preload" env:"SKIP_PRELOAD" description:"Skip preloading existing jobs. May cause duplicates in notifications after restart"`
}

func New(cfg Config, rule config.Rule, clientset *kubernetes.Clientset) *Engine {
	cache, err := simplelru.NewLRU(cfg.DedupCache, nil)
	if err != nil {
		panic(err)
	}
	return &Engine{
		rule:      rule,
		cfg:       cfg,
		clientset: clientset,
		cache:     cache,
	}
}

type Engine struct {
	cfg       Config
	rule      config.Rule
	clientset *kubernetes.Clientset
	cache     simplelru.LRUCache
}

func (e *Engine) Run(ctx context.Context) error {
	wg := pool.New().WithContext(ctx).WithCancelOnError()

	outputs := e.createUpstreams(wg)

	wg.Go(func(ctx context.Context) error {
		return e.watchJobs(ctx, outputs)
	})

	return wg.Wait()
}

func (e *Engine) watchJobs(ctx context.Context, outputs []upstreams.Upstream) error {
	var labelsFilter string
	if len(e.rule.Labels) > 0 {
		labelsFilter = labels.FormatLabels(e.rule.Labels)
	}

	listOpts := v1.ListOptions{
		LabelSelector: labelsFilter,
		Watch:         true,
	}

	if !e.cfg.SkipPreload {
		if err := e.preloadJobs(ctx, listOpts); err != nil {
			return fmt.Errorf("preload jobs: %w", err)
		}
	}

	slog.Info("watching jobs", "namespace", e.rule.Namespace, "labels", labelsFilter)
	observer, err := e.clientset.BatchV1().Jobs(e.rule.Namespace).Watch(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("create job watch: %w", err)
	}
	defer observer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-observer.ResultChan():
			if !ok {
				return fmt.Errorf("observer channel closed")
			}
			switch event.Type {
			case watch.Added, watch.Modified, watch.Error: // ignore bookmark
				job, ok := event.Object.(*v2.Job)
				if !ok {
					slog.Warn("event is not a job", "event", event)
					continue
				}

				if err := e.inspectJob(ctx, job, outputs); err != nil {
					slog.Warn("inspect job failed", "job", job.Name, "error", err)
				}
			}
		}
	}
}

func (e *Engine) preloadJobs(ctx context.Context, options v1.ListOptions) error {
	slog.Info("preloading jobs to avoid sending old notifications")
	cp := options
	cp.Watch = false
	jobs, err := e.clientset.BatchV1().Jobs(e.rule.Namespace).List(ctx, cp)
	if err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}
	for _, job := range jobs.Items {
		if job.Status.Failed == 0 {
			continue
		}

		e.cache.Add(job.UID, "")
	}
	slog.Info("discovered old jobs", "count", e.cache.Len())
	return nil
}

func (e *Engine) inspectJob(ctx context.Context, job *v2.Job, outputs []upstreams.Upstream) error {
	if job.Status.Failed == 0 {
		return nil
	}

	if e.cache.Contains(job.UID) {
		slog.Info("job already was discovered", "job", job.Name)
		return nil
	}

	e.cache.Add(job.UID, "")

	pods, err := e.fetchPods(ctx, job)
	if err != nil {
		return fmt.Errorf("fetch pods: %w", err)
	}
	slog.Info("found failed job", "job", job.Name, "namespace", job.Namespace)
	renderCtx := &config.RenderContext{
		Job:  job,
		Pods: pods,
	}
	var errList []error
	for _, upstream := range outputs {
		errList = append(errList, upstream.Send(ctx, renderCtx))
	}
	slog.Info("notifications enqueued", "job", job.Name, "namespace", job.Namespace)
	return errors.Join(errList...)
}

func (e *Engine) createUpstreams(wg *pool.ContextPool) []upstreams.Upstream {
	var all []upstreams.Upstream

	// initialize all upstreams here

	// webhooks
	for _, wh := range e.rule.Webhooks {
		up := upstreams.NewAsync(wh.UpstreamConfig, webhook.New(wh.Config))
		wg.Go(up.Run)
		all = append(all, up)
	}
	return all
}

func (e *Engine) fetchPods(ctx context.Context, job *v2.Job) ([]config.Pod, error) {
	pods, err := e.clientset.CoreV1().Pods(e.rule.Namespace).List(ctx, v1.ListOptions{
		LabelSelector: "job-name=" + job.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	var out = make([]config.Pod, 0, len(pods.Items))
	for _, pod := range pods.Items {
		logs, err := e.fetchLogs(ctx, &pod)
		if err != nil {
			// do not fail - empty logs are better than no notifications
			slog.Warn("fetch logs failed", "pod", pod.Name, "namespace", pod.Namespace, "error", err)
		}
		out = append(out, config.Pod{
			Pod:  &pod,
			Logs: logs,
		})
	}
	return out, nil
}

func (e *Engine) fetchLogs(ctx context.Context, pod *v3.Pod) (string, error) {
	logsRes := e.clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v3.PodLogOptions{
		TailLines: &e.cfg.Tail,
	})
	stream, err := logsRes.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("stream logs: %w", err)
	}
	content, err := io.ReadAll(io.LimitReader(stream, e.cfg.LogsBytes))
	_ = stream.Close()
	if err != nil {
		return "", fmt.Errorf("get pod %s logs: %w", pod.Name, err)
	}
	return string(content), nil
}
