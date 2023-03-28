export GOBIN=$(CURDIR)/bin
export PATH:=$(GOBIN):$(PATH)

include .bingo/Variables.mk

export GOROOT=$(shell go env GOROOT)
export GOFLAGS=-mod=vendor
export GO111MODULE=on
export CGO_ENABLED=0
export GOOS=linux

GOARCH ?= amd64

SHELL := /usr/bin/env bash
DOCKER_TAG ?= latest
DOCKER_IMG ?= quay.io/netobserv/flowlogs-pipeline
OCI_RUNTIME_PATH = $(shell which podman  || which docker)
OCI_RUNTIME ?= $(shell echo '$(OCI_RUNTIME_PATH)'|sed -e 's/.*\/\(.*\)/\1/g')
MIN_GO_VERSION := 1.18.0
FLP_BIN_FILE=flowlogs-pipeline
CG_BIN_FILE=confgenerator
PERF_BIN_FILE=perfmeasurements
NETFLOW_GENERATOR=nflow-generator
CMD_DIR=./cmd/
FLP_CONF_FILE ?= contrib/kubernetes/flowlogs-pipeline.conf.yaml
KIND_CLUSTER_NAME ?= kind

BUILD_DATE := $(shell date +%Y-%m-%d\ %H:%M)
TAG_COMMIT := $(shell git rev-list --abbrev-commit --tags --max-count=1)
TAG := $(shell git describe --abbrev=0 --tags ${TAG_COMMIT} 2>/dev/null || true)
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_VERSION := $(TAG:v%=%)
ifneq ($(COMMIT), $(TAG_COMMIT))
	BUILD_VERSION := $(BUILD_VERSION)-$(COMMIT)
endif
ifneq ($(shell git status --porcelain),)
	BUILD_VERSION := $(BUILD_VERSION)-dirty
endif

.DEFAULT_GOAL := help

FORCE: ;

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: validate_go
validate_go:
	@current_ver=$$(go version | { read _ _ v _; echo $${v#go}; }); \
	required_ver=${MIN_GO_VERSION}; min_ver=$$(echo -e "$$current_ver\n$$required_ver" | sort -V | head -n 1); \
	if [[ $$min_ver == $$current_ver ]]; then echo -e "\n!!! golang version > $$required_ver required !!!\n"; exit 7;fi

##@ Develop

.PHONY: validate_go lint
lint: $(GOLANGCI_LINT) ## Lint the code
	$(GOLANGCI_LINT) run --enable goimports --enable gofmt --enable ineffassign --timeout 5m

.PHONY: build_code
build_code:
	GOARCH=${GOARCH} go build -ldflags "-X 'main.BuildVersion=$(BUILD_VERSION)' -X 'main.BuildDate=$(BUILD_DATE)'" "${CMD_DIR}${FLP_BIN_FILE}"
	GOARCH=${GOARCH} go build -ldflags "-X 'main.BuildVersion=$(BUILD_VERSION)' -X 'main.BuildDate=$(BUILD_DATE)'" "${CMD_DIR}${CG_BIN_FILE}"

.PHONY: build
build: validate_go lint build_code docs ## Build flowlogs-pipeline executable and update the docs

.PHONY: perf
perf:
	GOARCH=${GOARCH} go build -ldflags "-X 'main.BuildVersion=$(BUILD_VERSION)' -X 'main.BuildDate=$(BUILD_DATE)'" "${CMD_DIR}${PERF_BIN_FILE}"

.PHONY: docs
docs: FORCE ## Update flowlogs-pipeline documentation
	@./hack/update-docs.sh
	@go run cmd/apitodoc/main.go > docs/api.md
	@go run cmd/operationalmetricstodoc/main.go > docs/operational-metrics.md

.PHONY: clean
clean: ## Clean
	rm -f "${FLP_BIN_FILE}"
	go clean ./...

# note: to review coverage execute: go tool cover -html=/tmp/coverage.out
TEST_OPTS := -race -coverpkg=./... -covermode=atomic -coverprofile=/tmp/coverage.out
.PHONY: tests-unit
tests-unit: validate_go ## Unit tests
	# enabling CGO is required for -race flag
	CGO_ENABLED=1 go test -p 1 $(TEST_OPTS) $$(go list ./... | grep -v /e2e)

.PHONY: tests-fast
tests-fast: TEST_OPTS=
tests-fast: tests-unit ## Fast unit tests (no race tests / coverage)

.PHONY: tests-e2e
tests-e2e: validate_go $(KIND)  ## End-to-end tests
	go test -p 1 -v -timeout 20m $$(go list ./... | grep  /e2e)

.PHONY: tests-all
tests-all: validate_go tests-unit tests-e2e ## All tests

# note: to review profile execute: go tool pprof -web /tmp/flowlogs-pipeline-cpu-profile.out (make sure graphviz is installed)
.PHONY: benchmarks
benchmarks: $(BENCHSTAT) validate_go ## Benchmark
	go test -bench=. ./cmd/flowlogs-pipeline -o=/tmp/flowlogs-pipeline.test \
	-cpuprofile /tmp/flowlogs-pipeline-cpu-profile.out \
	-run=^# -count=10 -parallel=1 -cpu=1 -benchtime=100x \
	 | tee /tmp/flowlogs-pipeline-benchmark.txt
	 $(BENCHSTAT) /tmp/flowlogs-pipeline-benchmark.txt

.PHONY: run
run: build ## Run
	./"${FLP_BIN_FILE}"

##@ Docker

# note: to build and push custom image tag use: DOCKER_TAG=test make push-image
.PHONY: build-image
build-image:
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) build -t $(DOCKER_IMG):$(DOCKER_TAG) -f contrib/docker/Dockerfile .

# It would have been better to have
build-single-multiarch-linux/%:
#The --load option is ignored by podman but required for docker
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) buildx build --load --build-arg TARGETPLATFORM=linux/$* --build-arg TARGETARCH=$* --build-arg BUILDPLATFORM=linux/amd64 -t $(DOCKER_IMG):$(DOCKER_TAG)-$* -f contrib/docker/Dockerfile .

# It would have been better to have
push-single-multiarch-linux/%: build-single-multiarch-linux/%
#The --load option is ignored by podman but required for docker
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) push $(DOCKER_IMG):$(DOCKER_TAG)-$*

# note: to build and push custom image tag use: DOCKER_TAG=test make push-image
.PHONY: build-multiarch-manifest
build-multiarch-manifest: push-single-multiarch-linux/amd64 push-single-multiarch-linux/arm64 push-single-multiarch-linux/ppc64le
	#if using Docker, image needs to be pushed before beeing added to the manifest
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) manifest create $(DOCKER_IMG):$(DOCKER_TAG) --amend $(DOCKER_IMG):$(DOCKER_TAG)-amd64 --amend $(DOCKER_IMG):$(DOCKER_TAG)-arm64 --amend $(DOCKER_IMG):$(DOCKER_TAG)-ppc64le

.PHONY: push-multiarch-manifest
push-multiarch-manifest: build-multiarch-manifest
	@echo 'publish manifest $(DOCKER_TAG) to $(DOCKER_IMG)'
ifeq (${OCI_RUNTIME} , docker)
		DOCKER_BUILDKIT=1 $(OCI_RUNTIME) manifest push $(DOCKER_IMG):$(DOCKER_TAG)
else
		DOCKER_BUILDKIT=1 $(OCI_RUNTIME) manifest push $(DOCKER_IMG):$(DOCKER_TAG) docker://$(DOCKER_IMG):$(DOCKER_TAG)
endif


.PHONY: build-ci-images
build-ci-images:
ifeq ($(DOCKER_TAG), main)
# Also tag "latest" only for branch "main"
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) build -t $(DOCKER_IMG):$(DOCKER_TAG) -t $(DOCKER_IMG):latest -f contrib/docker/Dockerfile .
else
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) build -t $(DOCKER_IMG):$(DOCKER_TAG) -f contrib/docker/Dockerfile .
endif
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) build --build-arg BASE_IMAGE=$(DOCKER_IMG):$(DOCKER_TAG) -t $(DOCKER_IMG):$(COMMIT) -f contrib/docker/shortlived.Dockerfile .

.PHONY: push-ci-images
push-ci-images: build-ci-images
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) push $(DOCKER_IMG):$(COMMIT)
ifeq ($(DOCKER_TAG), main)
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) push $(DOCKER_IMG):$(DOCKER_TAG)
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) push $(DOCKER_IMG):latest
endif

.PHONY: push-image
push-image: build-image ## Push latest image
	@echo 'publish image $(DOCKER_TAG) to $(DOCKER_IMG)'
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) push $(DOCKER_IMG):$(DOCKER_TAG)

##@ kubernetes

# note: to deploy with custom image tag use: DOCKER_TAG=test make deploy
.PHONY: deploy
deploy: ## Deploy the image
	sed 's|%DOCKER_IMG%|$(DOCKER_IMG)|g;s|%DOCKER_TAG%|$(DOCKER_TAG)|g' contrib/kubernetes/deployment.yaml > /tmp/deployment.yaml
	kubectl create configmap flowlogs-pipeline-configuration --from-file=flowlogs-pipeline.conf.yaml=$(FLP_CONF_FILE)
	kubectl apply -f /tmp/deployment.yaml
	kubectl rollout status "deploy/flowlogs-pipeline" --timeout=600s

.PHONY: undeploy
undeploy: ## Undeploy the image
	sed 's|%DOCKER_IMG%|$(DOCKER_IMG)|g;s|%DOCKER_TAG%|$(DOCKER_TAG)|g' contrib/kubernetes/deployment.yaml > /tmp/deployment.yaml
	kubectl --ignore-not-found=true  delete configmap flowlogs-pipeline-configuration || true
	kubectl --ignore-not-found=true delete -f /tmp/deployment.yaml || true

.PHONY: deploy-loki
deploy-loki: ## Deploy loki
	kubectl apply -f contrib/kubernetes/deployment-loki-storage.yaml
	kubectl apply -f contrib/kubernetes/deployment-loki.yaml
	kubectl rollout status "deploy/loki" --timeout=600s
	-pkill --oldest --full "3100:3100"
	kubectl port-forward --address 0.0.0.0 svc/loki 3100:3100 2>&1 >/dev/null &
	@echo -e "\nloki endpoint is available on http://localhost:3100\n"

.PHONY: undeploy-loki
undeploy-loki: ## Undeploy loki
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-loki.yaml || true
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-loki-storage.yaml || true
	-pkill --oldest --full "3100:3100"

.PHONY: deploy-prometheus
deploy-prometheus: ## Deploy prometheus
	kubectl apply -f contrib/kubernetes/deployment-prometheus.yaml
	kubectl rollout status "deploy/prometheus" --timeout=600s
	-pkill --oldest --full "9090:9090"
	kubectl port-forward --address 0.0.0.0 svc/prometheus 9090:9090 2>&1 >/dev/null &
	@echo -e "\nprometheus ui is available on http://localhost:9090\n"

.PHONY: undeploy-prometheus
undeploy-prometheus: ## Undeploy prometheus
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-prometheus.yaml || true
	-pkill --oldest --full "9090:9090"

.PHONY: deploy-grafana
deploy-grafana: ## Deploy grafana
	kubectl create configmap grafana-dashboard-definitions --from-file=contrib/dashboards/
	kubectl apply -f contrib/kubernetes/deployment-grafana.yaml
	kubectl rollout status "deploy/grafana" --timeout=600s
	-pkill --oldest --full "3000:3000"
	kubectl port-forward --address 0.0.0.0 svc/grafana 3000:3000 2>&1 >/dev/null &
	@echo -e "\ngrafana ui is available (user: admin password: admin) on http://localhost:3000\n"

.PHONY: undeploy-grafana
undeploy-grafana: ## Undeploy grafana
	kubectl --ignore-not-found=true  delete configmap grafana-dashboard-definitions || true
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-grafana.yaml || true
	-pkill --oldest --full "3000:3000"

.PHONY: deploy-netflow-simulator
deploy-netflow-simulator: ## Deploy netflow simulator
	kubectl apply -f contrib/kubernetes/deployment-netflow-simulator.yaml
	kubectl rollout status "deploy/netflowsimulator" --timeout=600s

.PHONY: undeploy-netflow-simulator
undeploy-netflow-simulator: ## Undeploy netflow simulator
	kubectl --ignore-not-found=true delete -f contrib/kubernetes/deployment-netflow-simulator.yaml || true

##@ kind

.PHONY: create-kind-cluster
create-kind-cluster: $(KIND) ## Create cluster
	$(KIND) create cluster --name $(KIND_CLUSTER_NAME) --config contrib/kubernetes/kind/kind.config.yaml
	kubectl cluster-info --context kind-kind

.PHONY: delete-kind-cluster
delete-kind-cluster: $(KIND) ## Delete cluster
	$(KIND) delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: kind-load-image
kind-load-image: ## Load image to kind
ifeq ($(OCI_RUNTIME),$(shell which docker))
# This is an optimization for docker provider. "kind load docker-image" can load an image directly from docker's
# local registry. For other providers (i.e. podman), we must use "kind load image-archive" instead.
	$(KIND) load --name $(KIND_CLUSTER_NAME) docker-image $(DOCKER_IMG):$(DOCKER_TAG)
else
	$(eval tmpfile="/tmp/flp.tar")
	-rm $(tmpfile)
	$(OCI_RUNTIME) save $(DOCKER_IMG):$(DOCKER_TAG) -o $(tmpfile)
	$(KIND) load --name $(KIND_CLUSTER_NAME) image-archive $(tmpfile)
	-rm $(tmpfile)
endif

##@ metrics

.PHONY: generate-configuration
generate-configuration: $(KIND) ## Generate metrics configuration
	go build "${CMD_DIR}${CG_BIN_FILE}"
	./${CG_BIN_FILE} --log-level debug --srcFolder network_definitions \
					--destConfFile $(FLP_CONF_FILE) \
					--destDocFile docs/metrics.md \
					--destGrafanaJsonnetFolder contrib/dashboards/jsonnet/

##@ End2End

.PHONY: local-deployments-deploy
local-deployments-deploy: $(KIND) deploy-prometheus deploy-loki deploy-grafana build-image kind-load-image deploy deploy-netflow-simulator
	kubectl get pods
	kubectl rollout status -w deployment/flowlogs-pipeline
	kubectl logs -l app=flowlogs-pipeline

.PHONY: local-deploy
local-deploy: $(KIND) local-cleanup create-kind-cluster local-deployments-deploy ## Deploy locally on kind (with simulated flowlogs)

.PHONY: local-deployments-cleanup
local-deployments-cleanup: $(KIND) undeploy-netflow-simulator undeploy undeploy-grafana undeploy-loki undeploy-prometheus

.PHONY: local-cleanup
local-cleanup: $(KIND) local-deployments-cleanup delete-kind-cluster ## Undeploy from local kind

.PHONY: local-redeploy
local-redeploy:  local-deployments-cleanup local-deployments-deploy ## Redeploy locally (on current kind)

.PHONY: ocp-deploy
ocp-deploy: ocp-cleanup deploy-prometheus deploy-loki deploy-grafana deploy ## Deploy to OCP
	flowlogs_pipeline_svc_ip=$$(kubectl get svc flowlogs-pipeline -o jsonpath='{.spec.clusterIP}'); \
	./hack/enable-ocp-flow-export.sh $$flowlogs_pipeline_svc_ip
	kubectl get pods
	kubectl rollout status -w deployment/flowlogs-pipeline
	kubectl logs -l app=flowlogs-pipeline
	oc expose service prometheus || true
	@prometheus_url=$$(oc get route prometheus -o jsonpath='{.spec.host}'); \
	echo -e "\nAccess prometheus on OCP using: http://"$$prometheus_url"\n"
	oc expose service grafana || true
	@grafana_url=$$(oc get route grafana -o jsonpath='{.spec.host}'); \
	echo -e "\nAccess grafana on OCP using: http://"$$grafana_url"\n"
	oc expose service loki || true
	@loki_url=$$(oc get route loki -o jsonpath='{.spec.host}'); \
	echo -e "\nAccess loki on OCP using: http://"$$loki_url"\n"

.PHONY: ocp-cleanup
ocp-cleanup: undeploy undeploy-loki undeploy-prometheus undeploy-grafana ## Undeploy from OCP

.PHONY: dev-local-deploy
dev-local-deploy: ## Deploy locally with simulated netflows
	-pkill --oldest --full "${FLP_BIN_FILE}"
	-pkill --oldest --full "${NETFLOW_GENERATOR}"
	go build "${CMD_DIR}${FLP_BIN_FILE}"
	go build "${CMD_DIR}${CG_BIN_FILE}"
	./${CG_BIN_FILE} --log-level debug --srcFolder network_definitions \
	--skipWithTags "kubernetes" \
	--destConfFile /tmp/flowlogs-pipeline.conf.yaml --destGrafanaJsonnetFolder /tmp/
	test -f /tmp/${NETFLOW_GENERATOR} || curl -L --output /tmp/${NETFLOW_GENERATOR}  https://github.com/nerdalert/nflow-generator/blob/master/binaries/nflow-generator-x86_64-linux?raw=true
	chmod +x /tmp/${NETFLOW_GENERATOR}
	./"${FLP_BIN_FILE}" --config /tmp/flowlogs-pipeline.conf.yaml &
	/tmp/${NETFLOW_GENERATOR} -t 127.0.0.1 -p 2056 > /dev/null 2>&1 &
