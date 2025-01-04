all: trigger

# Install kubernetes-in-docker (kind) tool
kind:
ifeq ($(shell command -v kind),)
	@echo Installing KIND
	go install sigs.k8s.io/kind@v0.26.0
else
	@echo KIND already installed
endif

.PHONY: kind

# Setup cluster
kubeconfig: kind
	@kubectl cluster-info --context kind-kind || kind create cluster
	@kind get kubeconfig > kubeconfig

cluster: kubeconfig
.PHONY: cluster

# Trigger job that will fail
trigger: cluster
	@kubectl --context kind-kind delete -f job.yaml || echo "nothing to delete"
	@kubectl --context kind-kind apply -f job.yaml
	@echo Mock job that should fail created
.PHONY: trigger

# Clean everything including cluster
clean:
	kind delete cluster
	rm -f kubeconfig
.PHONY: clean

# Run dev version of job-notifier using kind
run: kubeconfig
	@go run ../main.go -C kubeconfig
.PHONY: run

# Print all jobs
jobs: kubeconfig
	@kubectl --context kind-kind get jobs
.PHONY: jobs

# Print job logs
logs: kubeconfig
	@kubectl --context kind-kind logs job/failing-test-job
.PHONY: logs
