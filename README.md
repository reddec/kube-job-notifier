# Kube job notifier

Simple background service for Kubernetes which monitors failed jobs and sends notifications.

Similar projects:

- https://github.com/sukeesh/k8s-job-notify - has no cluster scope, no general webhooks

Inspired by prometheus and alertmanager.

Designed (but not limited to) for [NTFY](https://ntfy.sh/).

Supported upstreams (notification destinations):

- webhooks
- logger

Example [rule config](#rules) (supports multiple document in single file):

```yaml
---
# General webhook
webhooks:
  - url: https://webhook.site/f4cbc7d5-5e81-4eca-b0b2-ca6bfa75025c
```

## Roadmap

- [ ] SMTP (email) notification
- [ ] NATS notification
- [ ] (maybe) success notification

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

Rules are set of instructions what to watch and where to send notification.

Supported filters:

- namespace
- labels

Supported upstreams:

- webhook
- logger

Each rule is a YAML document stored in a single file. You may define multiple documents in one yaml file (using `---`
separator).

Rule has reasonable defaults, so minimal configuration will be:

```yaml
---
webhooks:
  - url: https://webhook.site/f4cbc7d5-5e81-4eca-b0b2-ca6bfa75025c
```

All notifications are delivered asynchronicity with individual in-memory queue. If notification failed it will be
retried several times with fixed time interval.
Queue is capped; if no notifications can be enqueued, new notifications will be dropped. Practically speaking it's
possible only if system generates a lot of failed jobs and upstream is unhealthy.

Configuration reference

```yaml
# Namespace to watch. If not set - all namespaces will be watched
# Default: empty
namespace: "<namespace>"

# Labels to filter jobs. If not set - all jobs will be watched
# Default: empty
labels:
  key: value
  key2: value2

# Upstreams configuration

# Webhooks list of destinations
# See webhooks section
webhooks: [ ]

# Logger list
logger: [ ]
```

#### Webhook

Deliver notification via HTTP request. If response code is not 2xx then operation will be marked as failed and new
attempt will be done later.

```yaml
# Queue size: maximum number of notifications that can wait before processing.
# Default: 100
queue: 100

# Maximum number of retries (in addition to first attempt).
# Default: 5
retries: 5

# Interval between retries.
# Default: 1s
interval: 1s

# URL of destination endpoint. REQUIRED.
# Templates can be used (see template section).
# URL may contain user:password section for basic auth if needed.
# Default: empty
url: "<URL>"

# HTTP method to use. There is no validation on method, any HTTP method can be used.
# Default: POST
method: "POST"

# HTTP headers to use.
# Values can use templates (see template section). But keys can be only static.
# Default: empty
headers:
  Content-Type: "text/plain"

# Request payload (body). Can use complex templates (see template section).
# Default: see bellow
body: |
  Job {{.Job.Name}}

  {{range .Pods}}{{.Name}}

  {{.Logs}}


  {{end}}
```

If body is not text, it will be treated as templated object (see Telegram example):

- every text field considered as template
- it recursively applied for each item in array or record in map
- object structure pre-reserved
- header `Content-Type: application/json` will be set if the header wasn't set by config

##### Examples

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

#### Logger

Send notification to logger. Used primarily for debugging.

```yaml
# Message to print
# Default: see bellow
message: |
  Job {{.Job.Name}}

  {{range .Pods}}{{.Name}}

  {{.Logs}}


  {{end}}
```

> `queue`, `retries` and `interval` can also be set for logger, but doesn't have any practical meaning.

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
