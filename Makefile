export GOBIN=$(CURDIR)/bin
export PATH:=$(GOBIN):$(PATH)

export GOROOT=$(shell go env GOROOT)
export GOFLAGS=-mod=vendor
export GO111MODULE=on
export CGO_ENABLED=0
export GOOS=linux

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= main

# Go architecture and targets images to build
GOARCH ?= amd64
MULTIARCH_TARGETS ?= amd64

# In CI, to be replaced by `netobserv`
IMAGE_ORG ?= $(USER)

# Image registry such as quay or docker
IMAGE_REGISTRY ?= quay.io

# IMAGE_TAG_BASE defines the namespace and part of the image name for remote images.
IMAGE_TAG_BASE ?= $(IMAGE_REGISTRY)/$(IMAGE_ORG)/flowlogs-pipeline

# Image URL to use all building/pushing image targets
IMAGE ?= $(IMAGE_TAG_BASE):$(VERSION)

# Image building tool (docker / podman) - docker is preferred in CI
OCI_BIN_PATH = $(shell which docker 2>/dev/null || which podman)
OCI_BIN ?= $(shell basename ${OCI_BIN_PATH})
OCI_BUILD_OPTS ?=

ifneq ($(CLEAN_BUILD),)
	BUILD_DATE := $(shell date +%Y-%m-%d\ %H:%M)
	BUILD_SHA := $(shell git rev-parse --short HEAD)
	LDFLAGS ?= -X 'main.buildVersion=${VERSION}-${BUILD_SHA}' -X 'main.buildDate=${BUILD_DATE}'
endif

GOLANGCI_LINT_VERSION = v1.61.0
KIND_VERSION = v0.22.0

FLP_BIN_FILE=flowlogs-pipeline
CG_BIN_FILE=confgenerator
NETFLOW_GENERATOR=nflow-generator
CMD_DIR=./cmd/
FLP_CONF_FILE ?= contrib/kubernetes/flowlogs-pipeline.conf.yaml
KIND_CLUSTER_NAME ?= kind

.DEFAULT_GOAL := help

FORCE: ;

# build a single arch target provided as argument
define build_target
	echo 'building image for arch $(1)'; \
	DOCKER_BUILDKIT=1 $(OCI_BIN) buildx build --load --build-arg LDFLAGS="${LDFLAGS}" --build-arg TARGETARCH=$(1) ${OCI_BUILD_OPTS} -t ${IMAGE}-$(1) -f contrib/docker/Dockerfile .;
endef

# push a single arch target image
define push_target
	echo 'pushing image ${IMAGE}-$(1)'; \
	DOCKER_BUILDKIT=1 $(OCI_BIN) push ${IMAGE}-$(1);
endef

# manifest create a single arch target provided as argument
define manifest_add_target
	echo 'manifest add target $(1)'; \
	DOCKER_BUILDKIT=1 $(OCI_BIN) manifest add ${IMAGE} ${IMAGE}-$(1);
endef

# extract a single arch target binary
define extract_target
	echo 'extracting binary from ${IMAGE}-$(1)'; \
	$(OCI_BIN) create --name flp ${IMAGE}-$(1); \
	$(OCI_BIN) cp flp:/app/flowlogs-pipeline ./release-assets/flowlogs-pipeline-${VERSION}-linux-$(1); \
	$(OCI_BIN) rm -f flp;
endef

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

.PHONY: prereqs
prereqs: ## Check if prerequisites are met, and install missing dependencies
	@echo "### Checking if prerequisites are met, and installing missing dependencies"
	GOFLAGS="" go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}

.PHONY: prereqs-kind
prereqs-kind: ## Check if prerequisites are met for running kind, and install missing dependencies
	@echo "### Checking if KIND prerequisites are met, and installing missing dependencies"
	GOFLAGS="" go install sigs.k8s.io/kind@${KIND_VERSION}

.PHONY: vendors
vendors: ## Check go vendors
	@echo "### Checking vendors"
	go mod tidy && go mod vendor

##@ Develop

.PHONY: lint
lint: prereqs ## Lint the code
	golangci-lint run ./... --timeout=3m

.PHONY: compile
compile: ## Compile main flowlogs-pipeline and config generator
	GOARCH=${GOARCH} go build "${CMD_DIR}${FLP_BIN_FILE}"
	GOARCH=${GOARCH} go build "${CMD_DIR}${CG_BIN_FILE}"

.PHONY: build
build: lint compile docs ## Build flowlogs-pipeline executable and update the docs

.PHONY: docs
docs: FORCE ## Update flowlogs-pipeline documentation
	@./hack/update-docs.sh
	@go run cmd/apitodoc/main.go > docs/api.md
	@./hack/update-enum-docs.sh
	@go run cmd/operationalmetricstodoc/main.go > docs/operational-metrics.md

.PHONY: clean
clean: ## Clean
	rm -f "${FLP_BIN_FILE}"
	go clean ./...

TEST_OPTS := -race -coverpkg=./... -covermode=atomic -coverprofile cover.out
.PHONY: tests-unit
tests-unit: ## Unit tests
	# tests may rely on non-thread safe libs such as go-ipfix => no -race flag
	go test $$(go list ./... | grep /testnorace)
	# enabling CGO is required for -race flag
	CGO_ENABLED=1 go test -p 1 $(TEST_OPTS) $$(go list ./... | grep -v /e2e | grep -v /testnorace)

.PHONY: coverage-report
coverage-report: ## Generate coverage report
	@echo "### Generating coverage report"
# Fix go cover conflicting with goyacc generated code
# See https://github.com/golang/go/issues/41222#issuecomment-3023898581
	sed -i -e '/yaccpar/d' cover.out
	go tool cover --func=./cover.out

.PHONY: coverage-report-html
coverage-report-html: ## Generate HTML coverage report
	@echo "### Generating HTML coverage report"
# Fix go cover conflicting with goyacc generated code
# See https://github.com/golang/go/issues/41222#issuecomment-3023898581
	sed -i -e '/yaccpar/d' cover.out
	go tool cover --html=./cover.out

.PHONY: tests-fast
tests-fast: TEST_OPTS=
tests-fast: tests-unit ## Fast unit tests (no race tests / coverage)

.PHONY: tests-e2e
tests-e2e: prereqs-kind  ## End-to-end tests
	go test -p 1 -v -timeout 20m $$(go list ./... | grep  /e2e)

.PHONY: tests-all
tests-all: tests-unit tests-e2e ## All tests

# note: to review profile execute: go tool pprof -web /tmp/flowlogs-pipeline-cpu-profile.out (make sure graphviz is installed)
.PHONY: benchmarks
benchmarks: $(BENCHSTAT) ## Benchmark
	go test -bench=. ./cmd/flowlogs-pipeline -o=/tmp/flowlogs-pipeline.test \
	-cpuprofile /tmp/flowlogs-pipeline-cpu-profile.out \
	-run=^# -count=10 -parallel=1 -cpu=1 -benchtime=100x \
	 | tee /tmp/flowlogs-pipeline-benchmark.txt
	 $(BENCHSTAT) /tmp/flowlogs-pipeline-benchmark.txt

.PHONY: run
run: build ## Run
	./"${FLP_BIN_FILE}"

##@ Images

# note: to build and push custom image tag use: IMAGE_ORG=myuser VERSION=dev make images
.PHONY: image-build
image-build: ## Build MULTIARCH_TARGETS images
	trap 'exit' INT; \
	$(foreach target,$(MULTIARCH_TARGETS),$(call build_target,$(target)))

.PHONY: image-push
image-push: ## Push MULTIARCH_TARGETS images
	trap 'exit' INT; \
	$(foreach target,$(MULTIARCH_TARGETS),$(call push_target,$(target)))

.PHONY: manifest-build
manifest-build: ## Build MULTIARCH_TARGETS manifest
	@echo 'building manifest $(IMAGE)'
	DOCKER_BUILDKIT=1 $(OCI_BIN) rmi ${IMAGE} -f || true
	DOCKER_BUILDKIT=1 $(OCI_BIN) manifest create ${IMAGE} $(foreach target,$(MULTIARCH_TARGETS), --amend ${IMAGE}-$(target));

.PHONY: manifest-push
manifest-push: ## Push MULTIARCH_TARGETS manifest
	@echo 'publish manifest $(IMAGE)'
ifeq (${OCI_BIN}, docker)
	DOCKER_BUILDKIT=1 $(OCI_BIN) manifest push ${IMAGE};
else
	DOCKER_BUILDKIT=1 $(OCI_BIN) manifest push ${IMAGE} docker://${IMAGE};
endif

.PHONY: extract-binaries
extract-binaries: ## Extract all MULTIARCH_TARGETS binaries
	trap 'exit' INT; \
	mkdir -p release-assets; \
	$(foreach target,$(MULTIARCH_TARGETS),$(call extract_target,$(target)))

.PHONY: goyacc
goyacc: ## Regenerate filters query langage
	@echo "### Regenerate filters query langage"
	GOFLAGS="" go install golang.org/x/tools/cmd/goyacc@v0.32.0
	goyacc -o pkg/dsl/expr.y.go pkg/dsl/expr.y

include .mk/development.mk
include .mk/shortcuts.mk
