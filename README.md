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

## Roadmap

- [ ] SMTP (email) notification
- [ ] NATS notification
- [ ] Success notification

## Deployment

Kustomize is preferred.

Download bundle

```bash
mkdir -p kube-job-notifier
cd kube-job-notifier
curl -L https://github.com/reddec/kube-job-notifier/releases/latest/download/deploy.tar.gz | tar -xzvf -  
```

Or just use files from [deploy](deploy) directory.

Both AMD64 and ARM64 supported.

## Configuration

### Deployment (service)

Default values are from container image (see [Dockerfile](Dockerfile)).

| Environment variable | Default value      | Description                                                                       |
|----------------------|--------------------|-----------------------------------------------------------------------------------|
| `CONFIG`             | `/etc/notify.yaml` | Path to rules file                                                                |
| `ENGINE_TAIL`        | 20                 | Number of last (tail) lines from logs                                             |
| `ENGINE_LOGS_BYTES`  | 65535              | Maximum number of bytes to read from logs. Acts as protection for service it self |
| `ENGINE_DEDUP_CACHE` | 8192               | Maximum number of entries (job UIDs) for deduplication                            |

Optional variables that in 99.99% are not required for production deployment

```
  -C, --kubeconfig=          Path to kubernetes config file to run service outside of cluster [$KUBECONFIG]
      --master=              Kuberentes master URL [$MASTER_URL]
      
      --engine.skip-preload  Skip preloading existing jobs. May cause duplicates in notifications after restart [$ENGINE_SKIP_PRELOAD]
```

### Rules


If body is not text, it will be treated as templated object: 
- every text field considered as template
- it recursively applied for each item in array or record in map
- object structure pre-reserved

#### Examples

**NTFY**

```yaml
---
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

**Telegram**

See [Bot API](https://core.telegram.org/bots/api#sendmessage)

```yaml
---
webhooks:
  - url: https://api.telegram.org/botXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/sendMessage
    body:
      chat_id: 11111111111
      text: |
        {{or .Job.Labels.project .Job.Name}} failed


        {{range .Pods}}
        ### Pod: {{.Name}}

        ```
        {{.Logs}}
        ```

        {{end}}
```

- Create token from [Bot Father](https://t.me/BotFather)
- Replace `XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX` to your token
- Replace `11111111111` in chat ID to your chat (you can find it in [Get IDs bot](https://t.me/getidsbot)).

> Instead of setting token in plain text, set value as environment variable (ex: `TELEGRAM_TOKEN`) in deployment and
> wire it via template function (ex: `url: https://api.telegram.org/bot{{env "TELEGRAM_TOKEN"}}/sendMessage`)

### Template

Based on [GoTemplates](https://pkg.go.dev/text/template). All functions
from [Sprig](https://masterminds.github.io/sprig/) are available.

Available context:

- `.Job` - Kubernetes [Job object](https://pkg.go.dev/k8s.io/api@v0.32.0/batch/v1#Job)
- `.Pods` - **array** of [Pod object](https://pkg.go.dev/k8s.io/api@v0.32.0/core/v1#Pod) and string field `.Logs`

Example:

    # {{or .Job.Labels.project .Job.Name}}    

    {{range .Pods}}
    ### Pod: {{.Name}}
    
    ```
    {{.Logs}}
    ```
    
    {{end}}

### Best practices

- If you need to pass secret to template (for example API token), do not set it in template as-is. Instead, pass it as
  env variable in manifest and [use it](https://masterminds.github.io/sprig/os.html) in template as
  `{{env "MY_SECRET_ENV"}}`

## Development

Requirements

- Go 1.23.4+
- Docker
- GoReleaser
- free will and desire to make life easier

> TIP: in [test-env](test-env) there is Makefile which can do most things for you.

Run locally:

- Setup you kubernetes cluster or use [kind](https://kind.sigs.k8s.io/)
- Setup webhook receiver. For example: https://webhook.site/#!/
- Create **notify.yaml** file (it wil be ignored by git)

```yaml
---
webhooks:
  - url: https://webhook.site/xxx-yy-zz
```

Obviously replace URL to the link from webhook receiver.

Then run service

```bash
go run main.go -C ~/.kube/config
```


Normally, with KIND:

- in one terminal: `make -C test-env run` to run service
- in another terminal: `make -C test-env trigger` to trigger failed job

To cleanup:

- `make -C test-env clean`
