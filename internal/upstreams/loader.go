package upstreams

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/reddec/kube-job-notifier/internal/config"
)

type mapFunc func(value *yaml.Node) (Upstream, error)

type Loader struct {
	upstreams map[string]mapFunc
}

func (l *Loader) Build(upstreams map[string][]config.Upstream) ([]*AsyncUpstream, error) {
	var ans []*AsyncUpstream

	for ns, configs := range upstreams {
		builder, ok := l.upstreams[ns]
		if !ok {
			return nil, fmt.Errorf("upstream %s not found", ns)
		}
		for i, upCfg := range configs {
			instance, err := builder(upCfg.Config)
			if err != nil {
				return nil, fmt.Errorf("build upstream %s (%d): %w", ns, i, err)
			}
			ans = append(ans, &AsyncUpstream{
				queue:    make(chan *config.RenderContext, upCfg.Queue),
				retries:  upCfg.Retries,
				interval: upCfg.Interval,
				next:     instance,
				kind:     ns,
			})
		}
	}

	return ans, nil
}

func Register[T any, U Upstream](l *Loader, namespace string, defaultValue func() T, builder func(cfg T) U) {
	if l.upstreams == nil {
		l.upstreams = make(map[string]mapFunc)
	}

	l.upstreams[namespace] = func(value *yaml.Node) (Upstream, error) {
		var def = defaultValue()
		if err := value.Decode(&def); err != nil {
			return nil, err
		}

		return builder(def), nil
	}
}
