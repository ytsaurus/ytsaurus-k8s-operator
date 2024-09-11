
# Image URL to use all building/pushing image targets.
IMG ?= ${OPERATOR_IMAGE}:${OPERATOR_TAG}

## Name of kind cluster for all kind-* targets
KIND_CLUSTER_NAME ?= ${USER}-yt-kind
export KIND_CLUSTER_NAME

## Path to kind cluster config.
KIND_CLUSTER_CONFIG =

## K8s context for kind cluster
KIND_KUBE_CONTEXT = kind-$(KIND_CLUSTER_NAME)

## K8s namespace for sample cluster.
YTSAURUS_NAMESPACE ?= ytsaurus-dev

## YTsaurus spec for sample cluster.
YTSAURUS_SPEC ?= config/samples/cluster_v1_cri.yaml

## Version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24.2

## YTsaurus operator image name.
OPERATOR_IMAGE_DEFAULT = ytsaurus/k8s-operator
OPERATOR_IMAGE = $(OPERATOR_IMAGE_DEFAULT)$(RELEASE_SUFFIX)

## YTsaurus operator image tag.
OPERATOR_TAG = 0.0.0-alpha

OPERATOR_CHART = ytop-chart
OPERATOR_CHART_NAME_DEFAULT = ytop-chart
OPERATOR_CHART_NAME = $(OPERATOR_CHART_NAME_DEFAULT)$(RELEASE_SUFFIX)
OPERATOR_CHART_CRDS = $(OPERATOR_CHART)/templates/crds
OPERATOR_INSTANCE = ytsaurus-dev

## K8s namespace for YTsaurus operator.
OPERATOR_NAMESPACE = ytsaurus-operator

ifdef RELEASE_VERSION
DOCKER_BUILD_ARGS += --build-arg VERSION="$(RELEASE_VERSION)"
else
DOCKER_BUILD_ARGS += --build-arg VERSION="$(OPERATOR_TAG)"
endif
DOCKER_BUILD_ARGS += --build-arg REVISION="$(shell git rev-parse HEAD)"
DOCKER_BUILD_ARGS += --build-arg BUILD_DATE="$(shell date -Iseconds -u)"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

## Tests parallelism.
GINKGO_PROCS ?= 1

GINKGO_FLAGS += --vv
GINKGO_FLAGS += --trace
GINKGO_FLAGS += --procs="$(GINKGO_PROCS)"
GINKGO_FLAGS += --timeout=1h
GINKGO_FLAGS += --poll-progress-after=5m
GINKGO_FLAGS += --poll-progress-interval=1m
GINKGO_FLAGS += --label-filter="$(GINKGO_LABEL_FILTER)"

##@ General

.PHONY: all
all: lint build helm-chart test ## Default target: make build, run linters and unit tests.

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
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@awk 'BEGIN {FS = " \??= "; printf "\n\033[1m%s\033[0m\n", "Options:"} /^## .*/ {C=C substr($$0, 4) " "} /^[A-Z0-9_]* \??= .*/ && C { printf "  \033[36m%s\033[0m = %*s %s\n", $$1, -50+length($$1), $$2, C} /^[^#]/ { C="" }  END {  }  ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:maxDescLen=80 webhook paths="{\"./api/...\" , \"./controllers/...\", \"./pkg/...\"}" output:crd:artifacts:config=config/crd/bases
	$(MAKE) docs/api.md

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="{\"./api/...\" , \"./controllers/...\", \"./pkg/...\"}"

docs/api.md: config/crd-ref-docs/config.yaml $(wildcard api/v1/*_types.go) | crd-ref-docs
	$(CRD_REF_DOCS) --config $< --renderer=markdown --source-path=api/v1 --output-path=$@

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" \
	go test -v ./... -coverprofile cover.out -timeout 1800s

.PHONY: test-e2e
test-e2e: manifests generate fmt vet envtest ginkgo ## Run e2e tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" \
	YTSAURUS_ENABLE_E2E_TESTS=true \
	$(GINKGO) $(GINKGO_FLAGS) ./test/e2e/... -coverprofile cover.out -timeout 1800s

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes.
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-generated
lint-generated: generate helm-chart ## Check that generated files are uptodate and committed.
	test -z "$(shell git status --porcelain api docs/api.md config ytop-chart)"

.PHONY: canonize
canonize: manifests generate fmt vet envtest ## Canonize test results.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" \
	CANONIZE=y \
	go test -v ./... -coverprofile cover.out

##@ K8s operations

KIND_CLUSTER_CREATE_FLAGS = -v 100 --wait 120s  --retain
ifneq (${KIND_CLUSTER_CONFIG},)
  KIND_CLUSTER_CREATE_FLAGS  += --config ${KIND_CLUSTER_CONFIG}
endif

.PHONY: kind-create-cluster
kind-create-cluster: kind ## Create kind kubernetes cluster.
	@if ! $(KIND) get clusters | grep -q $(KIND_CLUSTER_NAME); then \
		$(KIND) create cluster --name $(KIND_CLUSTER_NAME) $(KIND_CLUSTER_CREATE_FLAGS); \
	fi
	$(MAKE) k8s-install-cert-manager

.PHONY: kind-create-cluster-with-registry
kind-create-cluster-with-registry:
	$(MAKE) kind-create-cluster KIND_CLUSTER_CONFIG=config/kind/kind-with-registry.yaml

.PHONY: kind-use-context
kind-use-context: ## Switch kubectl default context and namespace.
	$(KUBECTL) config set-context $(KIND_KUBE_CONTEXT) --namespace $(YTSAURUS_NAMESPACE)
	$(KUBECTL) config use-context $(KIND_KUBE_CONTEXT)

.PHONY: kind-delete-cluster
kind-delete-cluster: kind ## Delete kind kubernetes cluster.
	@if $(KIND) get clusters | grep -q $(KIND_CLUSTER_NAME); then \
		$(KIND) delete cluster --name $(KIND_CLUSTER_NAME); \
	fi

# https://github.com/containerd/containerd/blob/main/docs/hosts.md
# https://distribution.github.io/distribution/about/configuration/
# https://kind.sigs.k8s.io/docs/user/local-registry/

REGISTRY_CONFIG_DIR=config/registry

REGISTRY_LOCAL_PORT = 5000
REGISTRY_LOCAL_ADDR = localhost:${REGISTRY_LOCAL_PORT}
REGISTRY_LOCAL_NAME = ${USER}-registry-localhost-${REGISTRY_LOCAL_PORT}

define REGISTRY_LOCAL_CONFIG
server = "http://${REGISTRY_LOCAL_NAME}:5000"

[host."http://${REGISTRY_LOCAL_NAME}:5000"]
  capabilities = ["pull", "resolve", "push"]
  skip_verify = true
endef
export REGISTRY_LOCAL_CONFIG

kind-create-local-registry: ## Create local docker registry for kind kubernetes cluster.
	docker run --name ${REGISTRY_LOCAL_NAME} -d --restart=always \
		--mount type=volume,src=${REGISTRY_LOCAL_NAME},dst=/var/lib/registry \
		-p "127.0.0.1:${REGISTRY_LOCAL_PORT}:5000" \
		registry:2
	docker network connect "kind" ${REGISTRY_LOCAL_NAME}
	mkdir -p ${REGISTRY_CONFIG_DIR}/${REGISTRY_LOCAL_ADDR}
	echo "$$REGISTRY_LOCAL_CONFIG" >${REGISTRY_CONFIG_DIR}/${REGISTRY_LOCAL_ADDR}/hosts.toml

kind-delete-local-registry: ## Delete local docker registry.
	docker rm -f ${REGISTRY_LOCAL_NAME}
	docker volume rm ${REGISTRY_LOCAL_NAME}
	rm -fr "${REGISTRY_CONFIG_DIR}/${REGISTRY_LOCAL_ADDR}"

TEST_IMAGES = \
	ytsaurus/ytsaurus-nightly:dev-23.1-28ccaedbf353b870bedafb6e881ecf386a0a3779 \
	ytsaurus/ytsaurus-nightly:dev-23.1-9779e0140ff73f5a786bd5362313ef9a74fcd0de \
	ytsaurus/ytsaurus-nightly:dev-23.2-9c50056eacfa4fe213798a5b9ee828ae3acb1bca
.PHONY: kind-load-test-images
kind-load-test-images:
	$(foreach img,$(TEST_IMAGES),docker pull -q $(img) && $(KIND) load docker-image --name $(KIND_CLUSTER_NAME) $(img);)

SAMPLE_IMAGES = \
	ytsaurus/ytsaurus:stable-23.1.0-relwithdebinfo
.PHONY: kind-load-sample-images
kind-load-sample-images:
	$(foreach img,$(SAMPLE_IMAGES),docker pull -q $(img) && $(KIND) load docker-image --name $(KIND_CLUSTER_NAME) $(img);)

.PHONY: k8s-install-cert-manager
k8s-install-cert-manager:
	@if ! $(KUBECTL) get namespace/cert-manager &>/dev/null; then \
		$(KUBECTL) apply --server-side -f "https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml"; \
		$(KUBECTL) -n cert-manager wait --timeout=60s --for=condition=available --all deployment; \
	fi

.PHONY: helm-install
helm-install: ## Install helm chart from sources.
	$(HELM) upgrade --install --wait $(OPERATOR_INSTANCE) $(OPERATOR_CHART) \
		-n $(OPERATOR_NAMESPACE) --create-namespace \
		--set controllerManager.manager.image.repository=${OPERATOR_IMAGE} \
		--set controllerManager.manager.image.tag=${OPERATOR_TAG}
	$(KUBECTL) -n $(OPERATOR_NAMESPACE) rollout restart deployment -l app.kubernetes.io/instance=$(OPERATOR_INSTANCE)

.PHONY: helm-kind-install
helm-kind-install: helm-chart docker-build ## Build docker image, load into kind and install helm chart.
	$(KIND) load docker-image --name $(KIND_CLUSTER_NAME) ${OPERATOR_IMAGE}:${OPERATOR_TAG}
	$(MAKE) helm-install

.PHONY: helm-minikube-install
helm-minikube-install: helm-chart ## Build docker image in minikube and install helm chart.
	eval $$(minikube docker-env) && docker build ${DOCKER_BUILD_ARGS} -t ${OPERATOR_IMAGE}:${OPERATOR_TAG} .
	$(MAKE) helm-install

.PHONY: helm-uninstall
helm-uninstall: ## Uninstal helm chart.
	$(HELM) uninstall -n $(OPERATOR_NAMESPACE) $(OPERATOR_INSTANCE)

.PHONY: kind-deploy-ytsaurus
kind-deploy-ytsaurus: ## Deploy sample ytsaurus cluster and all requirements.
	$(MAKE) kind-create-cluster
	$(MAKE) helm-kind-install
	-$(KUBECTL) create namespace $(YTSAURUS_NAMESPACE)
	$(KUBECTL) apply --server-side -n $(YTSAURUS_NAMESPACE) -f $(YTSAURUS_SPEC)
	$(KUBECTL) wait -n $(YTSAURUS_NAMESPACE) --timeout=10m --for=jsonpath='{.status.state}=Running' --all ytsaurus
	$(MAKE) kind-yt-info

.PHONY: kind-undeploy-ytsaurus
kind-undeploy-ytsaurus: ## Undeploy sample ytsaurus cluster.
	$(KUBECTL) get namespace $(YTSAURUS_NAMESPACE)
	-$(KUBECTL) -n $(YTSAURUS_NAMESPACE) delete ytsaurus --all
	-$(KUBECTL) -n $(YTSAURUS_NAMESPACE) delete pods --all --force
	$(KUBECTL) delete namespace $(YTSAURUS_NAMESPACE)

KIND_NODE_ADDR = $(shell docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $(1))
KIND_SERVICE_NODEPORT = $(shell $(KUBECTL) -n $(1) get service $(2) -o jsonpath="{.spec.ports[$(3)].nodePort}")

KIND_YT_ENV += YT_PROXY=http://$(call KIND_NODE_ADDR,${KIND_CLUSTER_NAME}-control-plane):$(call KIND_SERVICE_NODEPORT,${YTSAURUS_NAMESPACE},http-proxies-lb,0)
KIND_YT_ENV += YT_TOKEN=password
KIND_YT_ENV += YT_CONFIG_PATCHES="{ proxy={ enable_proxy_discovery=%false; }; operation_tracker={ always_show_job_stderr=%true; }; }"

kind-yt-env: ## Print yt cli environment for sample ytsaurus cluster.
	@printf "export \"%s\"\n" $(KIND_YT_ENV)

kind-yt-info:
	@printf "Kind k8s context: $(KIND_KUBE_CONTEXT) namespace: $(YTSAURUS_NAMESPACE)\nto set kubectl default context run: make kind-use-context\n\n"
	@printf "YTsaurus UI: http://%s:%s\nlogin/password: admin/password\nto setup env for yt cli run: . <(make kind-yt-env)\n" \
		$(call KIND_NODE_ADDR,${KIND_CLUSTER_NAME}-control-plane) \
		$(call KIND_SERVICE_NODEPORT,${YTSAURUS_NAMESPACE},ytsaurus-ui,0)

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build ${DOCKER_BUILD_ARGS} -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: helm-chart
helm-chart: manifests kustomize envsubst kubectl-slice ## Generate helm chart.
	$(KUSTOMIZE) build config/helm | name="$(OPERATOR_CHART)" $(ENVSUBST) | $(KUBECTL_SLICE) -q -o $(OPERATOR_CHART_CRDS) -t "{{.metadata.name}}.yaml" --prune
	name="$(OPERATOR_CHART_NAME)" version="$(RELEASE_VERSION)" $(ENVSUBST) < config/helm/Chart.yaml > $(OPERATOR_CHART)/Chart.yaml
	ytop_image_repo="$(OPERATOR_IMAGE)" $(ENVSUBST) < config/helm/values.yaml > $(OPERATOR_CHART)/values.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) create -f -

.PHONY: update
update: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) replace -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: reinstall
reinstall: uninstall install ## Reinstall CRDs from the K8s cluster specified in ~/.kube/config.

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) create -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

release: kustomize ## Release operator docker image and helm chart.
	docker build ${DOCKER_BUILD_ARGS} -t $(OPERATOR_IMAGE):${RELEASE_VERSION} .
	docker push $(OPERATOR_IMAGE):${RELEASE_VERSION}
	docker tag $(OPERATOR_IMAGE):${RELEASE_VERSION} ghcr.io/$(OPERATOR_IMAGE):${RELEASE_VERSION}
	docker push ghcr.io/$(OPERATOR_IMAGE):${RELEASE_VERSION}
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(OPERATOR_IMAGE):${RELEASE_VERSION}
	$(MAKE) helm-chart
	helm package $(OPERATOR_CHART)
	helm push $(OPERATOR_CHART_NAME)-${RELEASE_VERSION}.tgz oci://registry-1.docker.io/ytsaurus
	helm push $(OPERATOR_CHART_NAME)-${RELEASE_VERSION}.tgz oci://ghcr.io/ytsaurus

##@ Build Dependencies

## Location to install dependencies to.
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# Tool Binaries
KUBECTL ?= kubectl --context $(KIND_KUBE_CONTEXT)
HELM ?= helm --kube-context $(KIND_KUBE_CONTEXT)
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_GEN_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
GINKGO ?= $(LOCALBIN)/ginkgo-$(GINKGO_VERSION)
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs-$(CRD_REF_DOCS_VERSION)
KIND ?= $(LOCALBIN)/kind-$(KIND_VERSION)
ENVSUBST ?= $(LOCALBIN)/envsubst-$(ENVSUBST_VERSION)
KUBECTL_SLICE ?= $(LOCALBIN)/kubectl-slice-$(KUBECTL_SLICE_VERSION)

# Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_GEN_VERSION ?= v0.14.0
ENVTEST_VERSION ?= latest
## golangci-lint version.
GOLANGCI_LINT_VERSION ?= v1.56.2
GINKGO_VERSION ?= $(call go-get-version,github.com/onsi/ginkgo/v2)
CRD_REF_DOCS_VERSION ?= v0.0.12
## kind version.
KIND_VERSION ?= v0.22.0
CERT_MANAGER_VERSION ?= v1.14.4
ENVSUBST_VERSION ?= v1.4.2
KUBECTL_SLICE_VERSION ?= v1.3.1

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_GEN_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	$(call go-install-tool,$(GINKGO),github.com/onsi/ginkgo/v2/ginkgo,$(GINKGO_VERSION))

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	$(call go-install-tool,$(CRD_REF_DOCS),github.com/elastic/crd-ref-docs,$(CRD_REF_DOCS_VERSION))

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND): $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

.PHONY: envsubst
envsubst: $(ENVSUBST) ## Download envsubst locally if necessary.
$(ENVSUBST): $(LOCALBIN)
	$(call go-install-tool,$(ENVSUBST),github.com/a8m/envsubst/cmd/envsubst,$(ENVSUBST_VERSION))

.PHONY: kubectl-slice
kubectl-slice: $(KUBECTL_SLICE) ## Download kubectl-slice locally if necessary.
$(KUBECTL_SLICE): $(LOCALBIN)
	$(call go-install-tool,$(KUBECTL_SLICE),github.com/patrickdappollonio/kubectl-slice,$(KUBECTL_SLICE_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef

# go-get-version will retrieve version of module $1 from go.mod
go-get-version = $(shell go list -m $1 | awk '{print $$2}')
