# Kube job notifier

Simple background service for Kubernetes which monitors failed jobs and sends notifications.

Similar projects:

- https://github.com/sukeesh/k8s-job-notify - has no cluster scope, no general webhooks

Inspired by prometheus and alertmanager.

Designed (but not limited to) for [NTFY](https://ntfy.sh/).

Supported:

- webhooks

Example [rule config](#rules) (supports multiple document in single file):

```yaml
---
# General webhook
webhooks:
  - url: https://webhook.site/f4cbc7d5-5e81-4eca-b0b2-ca6bfa75025c
---
# Integration with NTFY
webhooks:
  - url: https://ntfy.example.com/cron-jobs
    headers:
      Title: "{{or .Job.Labels.project .Job.Name}} failed"
      Markdown: 'yes'
      Priority: "high"
      Tags': 'rotating_light'
    body: |
      {{range .Pods}}
      ### Pod: {{.Name}}

      ```
      {{.Logs}}
      ```

      {{end}}
```

## Deployment

Kustomize is preferred.

Supports cluster-scoped and namespaced roles. Download one of sets of manifests bellow and
then set specific version in `kustomization.yaml`.

**cluster-wide**

```bash
curl 
```

**namespaced**

```bash
curl 
```

## Configuration


### Deployment (service)



### Rules

