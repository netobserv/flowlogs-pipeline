export GOBIN=$(CURDIR)/bin
export PATH:=$(GOBIN):$(PATH)

include .bingo/Variables.mk

export GOROOT=$(shell go env GOROOT)
export GOFLAGS=-mod=vendor
export GO111MODULE=on
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=1

SHELL := /bin/bash
DOCKER_TAG ?= latest
DOCKER_IMG ?= quay.io/netobserv/flowlogs2metrics
OCI_RUNTIME ?= $(shell which podman  || which docker)
MIN_GO_VERSION := 1.17.0
FL2M_BIN_FILE=flowlogs2metrics
CG_BIN_FILE=confgenerator
NETFLOW_GENERATOR=nflow-generator
CMD_DIR=./cmd/
FL2M_CONF_FILE ?= contrib/kubernetes/flowlogs2metrics.conf.yaml

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
	@go mod vendor
	@$(GOLANGCI_LINT) run --timeout 5m

.PHONY: build_code
build_code: validate_go lint
	@go mod vendor
	VERSION=$$(date); go build -ldflags "-X 'main.Version=$$VERSION'" "${CMD_DIR}${FL2M_BIN_FILE}"

.PHONY: build
build: build_code docs ## Build flowlogs2metrics executable and update the docs

# Note: To change dashboards, change `dashboards.jsonnet`. Do not change manually `dashboards.json`
.PHONY: dashboards
dashboards: $(JB) $(JSONNET) ## Build grafana dashboards
	./hack/generate-dashboards.sh

.PHONY: docs
docs: FORCE ## Update flowlogs2metrics documentation
	@./hack/update-docs.sh
	@go run cmd/apitodoc/main.go > docs/api.md
.PHONY: clean
clean: ## Clean
	rm -f "${FL2M_BIN_FILE}"
	go clean ./...

# note: to review coverage execute: go tool cover -html=/tmp/coverage.out
.PHONY: test
test: validate_go ## Test
	go test -race -covermode=atomic -coverprofile=/tmp/coverage.out ./...

# note: to review profile execute: go tool pprof -web /tmp/flowlogs2metrics-cpu-profile.out (make sure graphviz is installed)
.PHONY: benchmarks
benchmarks: $(BENCHSTAT) validate_go ## Benchmark
	go test -bench=. ./cmd/flowlogs2metrics -o=/tmp/flowlogs2metrics.test \
	-cpuprofile /tmp/flowlogs2metrics-cpu-profile.out \
	-run=^# -count=10 -parallel=1 -cpu=1 -benchtime=100x \
	 | tee /tmp/flowlogs2metrics-benchmark.txt
	 $(BENCHSTAT) /tmp/flowlogs2metrics-benchmark.txt

.PHONY: run
run: build ## Run
	./"${FL2M_BIN_FILE}"

##@ Docker

# note: to build and push custom image tag use: DOCKER_TAG=test make push-image
.PHONY: build-image
build-image:
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) build -t $(DOCKER_IMG):$(DOCKER_TAG) -f contrib/docker/Dockerfile .

.PHONY: push-image
push-image: build-image ## Push latest image
	@echo 'publish image $(DOCKER_TAG) to $(DOCKER_IMG)'
	DOCKER_BUILDKIT=1 $(OCI_RUNTIME) push $(DOCKER_IMG):$(DOCKER_TAG)

##@ kubernetes

# note: to deploy with custom image tag use: DOCKER_TAG=test make deploy
.PHONY: deploy
deploy: ## Deploy the image
	sed 's|%DOCKER_IMG%|$(DOCKER_IMG)|g;s|%DOCKER_TAG%|$(DOCKER_TAG)|g' contrib/kubernetes/deployment.yaml > /tmp/deployment.yaml
	kubectl create configmap flowlogs2metrics-configuration --from-file=flowlogs2metrics.conf.yaml=$(FL2M_CONF_FILE)
	kubectl apply -f /tmp/deployment.yaml
	kubectl rollout status "deploy/flowlogs2metrics" --timeout=600s

.PHONY: undeploy
undeploy: ## Undeploy the image
	sed 's|%DOCKER_IMG%|$(DOCKER_IMG)|g;s|%DOCKER_TAG%|$(DOCKER_TAG)|g' contrib/kubernetes/deployment.yaml > /tmp/deployment.yaml
	kubectl --ignore-not-found=true  delete configmap flowlogs2metrics-configuration || true
	kubectl --ignore-not-found=true delete -f /tmp/deployment.yaml || true

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
	$(KIND) create cluster
	kubectl cluster-info --context kind-kind

.PHONY: delete-kind-cluster
delete-kind-cluster: $(KIND) ## Delete cluster
	$(KIND) delete cluster

##@ metrics

.PHONY: generate-configuration
generate-configuration: $(KIND) ## Generate metrics configuration
	go build "${CMD_DIR}${CG_BIN_FILE}"
	./${CG_BIN_FILE} --log-level debug --srcFolder network_definitions \
					--destConfFile $(FL2M_CONF_FILE) \
					--destDocFile docs/metrics.md \
					--destGrafanaJsonnetFolder contrib/dashboards/jsonnet/

##@ End2End

.PHONY: local-deployments-deploy
local-deployments-deploy: $(KIND) deploy-prometheus deploy-grafana deploy deploy-netflow-simulator
	kubectl get pods
	kubectl rollout status -w deployment/flowlogs2metrics
	kubectl logs -l app=flowlogs2metrics

.PHONY: local-deploy
local-deploy: $(KIND) local-cleanup create-kind-cluster local-deployments-deploy ## Deploy locally on kind (with simulated flowlogs)

.PHONY: local-deployments-cleanup
local-deployments-cleanup: $(KIND) undeploy-netflow-simulator undeploy undeploy-grafana undeploy-prometheus

.PHONY: local-cleanup
local-cleanup: $(KIND) local-deployments-cleanup delete-kind-cluster ## Undeploy from local kind

.PHONY: local-redeploy
local-redeploy:  local-deployments-cleanup local-deployments-deploy ## Redeploy locally (on current kind)

.PHONY: ocp-deploy
ocp-deploy: ocp-cleanup deploy-prometheus deploy-grafana deploy ## Deploy to OCP
	flowlogs2metrics_svc_ip=$$(kubectl get svc flowlogs2metrics -o jsonpath='{.spec.clusterIP}'); \
	./hack/enable-ocp-flow-export.sh $$flowlogs2metrics_svc_ip
	kubectl get pods
	kubectl rollout status -w deployment/flowlogs2metrics
	kubectl logs -l app=flowlogs2metrics
	oc expose service prometheus || true
	@prometheus_url=$$(oc get route prometheus -o jsonpath='{.spec.host}'); \
	echo -e "\nAccess prometheus on OCP using: http://"$$prometheus_url"\n"
	oc expose service grafana || true
	@grafana_url=$$(oc get route grafana -o jsonpath='{.spec.host}'); \
	echo -e "\nAccess grafana on OCP using: http://"$$grafana_url"\n"

.PHONY: ocp-cleanup
ocp-cleanup: undeploy undeploy-prometheus undeploy-grafana ## Undeploy from OCP

.PHONY: dev-local-deploy
dev-local-deploy: ## Deploy locally with simulated netflows
	-pkill --oldest --full "${FL2M_BIN_FILE}"
	-pkill --oldest --full "${NETFLOW_GENERATOR}"
	@go mod vendor
	go build "${CMD_DIR}${FL2M_BIN_FILE}"
	go build "${CMD_DIR}${CG_BIN_FILE}"
	./${CG_BIN_FILE} --log-level debug --srcFolder network_definitions \
	--skipWithLabels "kubernetes" \
	--destConfFile /tmp/flowlogs2metrics.conf.yaml --destGrafanaJsonnetFolder /tmp/
	test -f /tmp/${NETFLOW_GENERATOR} || curl -L --output /tmp/${NETFLOW_GENERATOR}  https://github.com/nerdalert/nflow-generator/blob/master/binaries/nflow-generator-x86_64-linux?raw=true
	chmod +x /tmp/${NETFLOW_GENERATOR}
	./"${FL2M_BIN_FILE}" --config /tmp/flowlogs2metrics.conf.yaml &
	/tmp/${NETFLOW_GENERATOR} -t 127.0.0.1 -p 2056 > /dev/null 2>&1 &
