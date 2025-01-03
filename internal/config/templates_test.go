package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/batch/v1"
	v2 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/reddec/kube-job-notifier/internal/config"
)

func TestParseJSON(t *testing.T) {

	const data = `{"headers": {"foo": "bar"}}`
	rf, err := config.NewComplexTemplate(map[string]any{
		"JOB-API": "{{.Job.APIVersion}}",
		"Fields":  []any{"foo", "bar", "{{.Job.Kind}}", map[string]string{"foo": "bar"}},
	})
	require.NoError(t, err)
	out, err := rf(&config.RenderContext{
		Job: &v1.Job{TypeMeta: v2.TypeMeta{
			APIVersion: "batch/v1", Kind: "Job",
		}},
		Pods: nil,
	})
	require.NoError(t, err)
	t.Logf("%+v", out)
}
