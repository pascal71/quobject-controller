# Makefile — Kubernetes Controller with ko and multi-arch support
# -------------------------------------------------
# Usage:
#   make                          # generate deepcopy + build
#   make ko-build                 # build multi-arch image with ko
#   make ko-resolve               # generate kubernetes manifests
#   make manifests                # generate CRD manifests
#   make install                  # install CRDs to cluster
#   make run                      # run controller locally
#   make deploy                   # deploy to cluster
#   make undeploy                 # remove from cluster

# -------------------------------------------------
# Configuration Variables
# -------------------------------------------------

# Go configuration
GO                     ?= go
GO_VERSION             ?= 1.24

# controller-gen configuration
CONTROLLER_GEN         ?= controller-gen
CONTROLLER_GEN_VERSION ?= v0.19.0

# Build configuration
BINARY                 ?= bin/manager
PKG_MAIN              ?= main.go

# Docker/Registry configuration (for traditional docker builds)
DOCKER_REGISTRY       ?= quay.io
DOCKER_ORG            ?= pamvdam
IMAGE_NAME            ?= quobject-controller
IMAGE_TAG             ?= latest
IMG                   ?= $(DOCKER_REGISTRY)/$(DOCKER_ORG)/$(IMAGE_NAME):$(IMAGE_TAG)

# ko configuration
KO_DOCKER_REPO        ?= $(DOCKER_REGISTRY)/$(DOCKER_ORG)/$(IMAGE_NAME)
KO_PLATFORMS          ?= linux/amd64,linux/arm64
KO_FLAGS              ?= --bare --tags=$(IMAGE_TAG)

# Kubernetes configuration
NAMESPACE             ?= quobject-controller
KUSTOMIZE_DIR         ?= config/default

# Optional: Skip code generation
# NOOP_GENERATE        ?= 1

# -------------------------------------------------
# Tool Detection and Installation
# -------------------------------------------------

# Check if controller-gen is installed locally
CONTROLLER_GEN_LOCAL := $(shell which $(CONTROLLER_GEN) 2>/dev/null)

# Determine which controller-gen to use
ifdef CONTROLLER_GEN_LOCAL
	CONTROLLER_GEN_CMD = $(CONTROLLER_GEN)
else
	CONTROLLER_GEN_CMD = $(GO) run sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
endif

# Check if ko is installed
KO_LOCAL := $(shell which ko 2>/dev/null)

# -------------------------------------------------
# Main Targets
# -------------------------------------------------

.PHONY: all
all: build

.PHONY: help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Development:"
	@echo "  make build          - Build the controller binary locally"
	@echo "  make generate       - Generate deepcopy code"
	@echo "  make manifests      - Generate CRD manifests"
	@echo "  make run            - Run controller locally"
	@echo "  make test           - Run tests"
	@echo ""
	@echo "Container builds with ko:"
	@echo "  make ko-build       - Build and push multi-arch container image"
	@echo "  make ko-resolve     - Generate Kubernetes manifests with image refs"
	@echo "  make ko-apply       - Build and deploy to cluster (requires kubectl)"
	@echo ""
	@echo "Traditional Docker builds:"
	@echo "  make docker-build   - Build docker image (single arch)"
	@echo "  make docker-push    - Push docker image"
	@echo "  make docker-buildx  - Build and push multi-arch with buildx"
	@echo ""
	@echo "Deployment:"
	@echo "  make install        - Install CRDs into cluster"
	@echo "  make deploy         - Deploy controller to cluster"
	@echo "  make undeploy       - Remove controller from cluster"
	@echo ""
	@echo "Configuration:"
	@echo "  KO_DOCKER_REPO=$(KO_DOCKER_REPO)"
	@echo "  KO_PLATFORMS=$(KO_PLATFORMS)"
	@echo "  NAMESPACE=$(NAMESPACE)"
	@echo "  GO_VERSION=$(GO_VERSION)"
	@echo "  CONTROLLER_GEN_VERSION=$(CONTROLLER_GEN_VERSION)"

# -------------------------------------------------
# Development Targets
# -------------------------------------------------

.PHONY: generate
ifeq ($(NOOP_GENERATE),1)
generate:
	@echo ">> Skipping code generation (NOOP_GENERATE=1)"
else
generate:
	@echo ">> Generating deepcopy code with controller-gen $(CONTROLLER_GEN_VERSION)"
	$(CONTROLLER_GEN_CMD) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."
endif

.PHONY: manifests
manifests:
	@echo ">> Generating CRD manifests with controller-gen $(CONTROLLER_GEN_VERSION)"
	$(CONTROLLER_GEN_CMD) crd paths="./api/..." output:crd:dir=config/crd/bases

.PHONY: build
build: generate
	@echo ">> Building $(BINARY)"
	GOFLAGS=-trimpath CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o $(BINARY) $(PKG_MAIN)

.PHONY: run
run: generate manifests
	@echo ">> Running controller locally"
	$(GO) run $(PKG_MAIN) \
		--metrics-bind-address=:8080 \
		--health-probe-bind-address=:8081

.PHONY: test
test: generate
	@echo ">> Running tests"
	$(GO) test -v -coverprofile=coverage.out ./...
	@echo ">> Coverage summary:"
	@$(GO) tool cover -func=coverage.out | tail -1

.PHONY: test-integration
test-integration: generate
	@echo ">> Running integration tests"
	$(GO) test -v ./test/integration/... -tags=integration

# -------------------------------------------------
# ko Build Targets (Recommended for Go apps in Kubernetes)
# -------------------------------------------------

.PHONY: ko-check
ko-check:
ifndef KO_LOCAL
	@echo ">> ko is not installed. Installing ko..."
	$(GO) install github.com/google/ko@latest
endif

.PHONY: ko-build
ko-build: ko-check manifests
	@echo ">> Building multi-arch container with ko"
	@echo ">> Registry: $(KO_DOCKER_REPO)"
	@echo ">> Platforms: $(KO_PLATFORMS)"
	KO_DOCKER_REPO=$(KO_DOCKER_REPO) ko build . \
		--platform=$(KO_PLATFORMS) \
		$(KO_FLAGS)

.PHONY: ko-resolve
ko-resolve: ko-check manifests
	@echo ">> Generating Kubernetes manifests with ko"
	@echo ">> Output: dist/release.yaml"
	@mkdir -p dist
	KO_DOCKER_REPO=$(KO_DOCKER_REPO) ko resolve \
		--platform=$(KO_PLATFORMS) \
		$(KO_FLAGS) \
		-f $(KUSTOMIZE_DIR) > dist/release.yaml
	@echo ">> Generated multi-arch release manifest at dist/release.yaml"
	@echo ">> Image will be at: $(KO_DOCKER_REPO)"

.PHONY: ko-apply
ko-apply: ko-check manifests
	@echo ">> Building and deploying with ko"
	@echo ">> Registry: $(KO_DOCKER_REPO)"
	@echo ">> Platforms: $(KO_PLATFORMS)"
	@echo ">> Target cluster: $$(kubectl config current-context)"
	@echo ">> Installing CRDs first"
	kubectl apply -f config/crd/bases/
	@echo ">> Deploying controller"
	kustomize build config/default | \
		KO_DOCKER_REPO=$(KO_DOCKER_REPO) ko resolve \
		--platform=$(KO_PLATFORMS) \
		$(KO_FLAGS) \
		-f - | kubectl apply -f -

.PHONY: ko-build-local
ko-build-local: ko-check
	@echo ">> Building local image with ko (for testing)"
	ko build . --local --platform=$(shell go env GOOS)/$(shell go env GOARCH)

# -------------------------------------------------
# Traditional Docker Targets (Alternative to ko)
# -------------------------------------------------

.PHONY: docker-build
docker-build:
	@echo ">> Building docker image: $(IMG)"
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push:
	@echo ">> Pushing docker image: $(IMG)"
	docker push $(IMG)

.PHONY: docker-buildx
docker-buildx:
	@echo ">> Building multi-arch image with buildx: $(IMG)"
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMG) --push .

.PHONY: docker-buildx-setup
docker-buildx-setup:
	@echo ">> Setting up Docker buildx for multi-arch"
	docker buildx create --name quobject-builder --use || true
	docker buildx inspect --bootstrap

# -------------------------------------------------
# Installation and Deployment Targets
# -------------------------------------------------

.PHONY: install
install: manifests
	@echo ">> Installing CRDs into cluster"
	kubectl apply -f config/crd/bases

.PHONY: uninstall
uninstall: manifests
	@echo ">> Uninstalling CRDs from cluster"
	kubectl delete --ignore-not-found -f config/crd/bases

.PHONY: deploy
deploy: ko-apply
	@echo ">> Controller deployed with ko"

.PHONY: deploy-docker
deploy-docker: manifests docker-push
	@echo ">> Deploying controller using Docker image"
	cd config/manager && kustomize edit set image controller=$(IMG)
	kubectl apply -k $(KUSTOMIZE_DIR)

.PHONY: undeploy
undeploy:
	@echo ">> Removing controller from cluster"
	kubectl delete --ignore-not-found -k $(KUSTOMIZE_DIR)

# -------------------------------------------------
# Maintenance Targets
# -------------------------------------------------

.PHONY: tidy
tidy:
	@echo ">> Running go mod tidy"
	$(GO) mod tidy

.PHONY: fmt
fmt:
	@echo ">> Formatting code"
	$(GO) fmt ./...
	gofmt -s -w .

.PHONY: vet
vet:
	@echo ">> Running go vet"
	$(GO) vet ./...

.PHONY: lint
lint:
	@echo ">> Running golangci-lint"
	golangci-lint run ./...

.PHONY: clean
clean:
	@echo ">> Cleaning build artifacts"
	rm -rf $(BINARY) bin/ dist/ coverage.out

.PHONY: clean-all
clean-all: clean
	@echo ">> Cleaning all generated files"
	rm -f api/v1alpha1/zz_generated.deepcopy.go
	rm -rf config/crd/bases/*.yaml

# -------------------------------------------------
# Setup and Utility Targets
# -------------------------------------------------

.PHONY: tools
tools:
	@echo ">> Installing development tools"
	$(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
	$(GO) install github.com/google/ko@latest
	$(GO) install sigs.k8s.io/kustomize/kustomize/v5@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/google/go-containerregistry/cmd/crane@latest

.PHONY: verify-tools
verify-tools:
	@echo ">> Tool versions:"
	@echo "Go: $(shell $(GO) version)"
	@echo "controller-gen: $(shell $(CONTROLLER_GEN_CMD) --version 2>/dev/null || echo 'not installed')"
	@echo "ko: $(shell ko version 2>/dev/null || echo 'not installed')"
	@echo "kubectl: $(shell kubectl version --client -o yaml | grep gitVersion || echo 'not installed')"
	@echo "docker: $(shell docker version --format '{{.Client.Version}}' 2>/dev/null || echo 'not installed')"

.PHONY: create-namespace
create-namespace:
	@echo ">> Creating namespace $(NAMESPACE)"
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -

.PHONY: create-s3-secret
create-s3-secret:
	@echo ">> Creating S3 credentials secret in namespace $(NAMESPACE)"
	@kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f - 2>/dev/null || true
	@echo "Enter S3 endpoint: " && read endpoint && \
	echo "Enter S3 region: " && read region && \
	echo "Enter S3 access key: " && read access_key && \
	echo "Enter S3 secret key: " && read -s secret_key && echo && \
	kubectl create secret generic s3-credentials \
		--namespace=$(NAMESPACE) \
		--from-literal=endpoint=$$endpoint \
		--from-literal=region=$$region \
		--from-literal=accessKey=$$access_key \
		--from-literal=secretKey=$$secret_key \
		--dry-run=client -o yaml | kubectl apply -f -

# -------------------------------------------------
# Development Helpers
# -------------------------------------------------

.PHONY: example
example:
	@echo ">> Creating example QuObjectBucketClaim"
	@kubectl apply -f - <<EOF
	apiVersion: quobject.io/v1alpha1
	kind: QuObjectBucketClaim
	metadata:
	  name: example-bucket-claim
	  namespace: default
	spec:
	  generateBucketName: example
	  storageClassName: standard
	EOF

.PHONY: logs
logs:
	@echo ">> Tailing controller logs"
	kubectl logs -n $(NAMESPACE) -l control-plane=controller-manager -f --tail=100

.PHONY: describe
describe:
	@echo ">> Describing QuObjectBucketClaims"
	kubectl describe quobjectbucketclaims -A

.PHONY: verify-image
verify-image:
	@echo ">> Verifying multi-arch image at $(KO_DOCKER_REPO)"
	@if command -v crane >/dev/null 2>&1; then \
		crane manifest $(KO_DOCKER_REPO):$(IMAGE_TAG) | jq '.manifests[] | {platform: .platform, digest: .digest}'; \
	else \
		echo "Install crane with: make tools"; \
		echo "Falling back to docker manifest inspect:"; \
		docker manifest inspect $(KO_DOCKER_REPO):$(IMAGE_TAG) 2>/dev/null || echo "Image not found or not a manifest list"; \
	fi

# -------------------------------------------------
# Quick Start Targets
# -------------------------------------------------

.PHONY: quickstart
quickstart: tools manifests install create-namespace create-s3-secret ko-apply
	@echo ">> QuObject Controller deployed successfully!"
	@echo ">> Check status with: make logs"
	@echo ">> Create a test bucket claim with: make example"

.PHONY: quickstart-local
quickstart-local: tools manifests install create-namespace create-s3-secret run
	@echo ">> Running QuObject Controller locally"
	@echo ">> Create a test bucket claim in another terminal with: make example"
