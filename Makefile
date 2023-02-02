SHELL = /bin/sh -e
AUTHOR_EMAIL=info@appvia.io
REGISTRY=ghcr.io
REGISTRY_ORG=appvia
APIS ?= $(shell find pkg/apis -name "v*" -type d | sed -e 's/pkg\/apis\///' | sort | tr '\n' ' ')
BUILD_TIME=$(shell date '+%s')
CLI_PLATFORMS=darwin linux windows
CLI_ARCHITECTURES=amd64 arm64
CURRENT_TAG=$(shell git tag --points-at HEAD)
DEPS=$(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
DOCKER_IMAGES ?= controller
GIT_REMOTE?=origin
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
GOVERSION ?= 1.17
HARDWARE=$(shell uname -m)
PLATFORM=$(shell uname -s)
PACKAGES=$(shell go list ./...)
PACKAGES_LIST=$(shell go list ./... | sed -e 's/github.com\/appvia\/terranetes-controller\//.\//')
GIT_SHA=$(shell git --no-pager describe --always --dirty)
BUILD_TIME=$(shell date '+%s')
GO_DIRS=cmd hack pkg
SH_DIRS=.circleci hack
ROOT_DIR=${PWD}
UNAME := $(shell uname)
LFLAGS ?= -X github.com/appvia/terranetes-controller/pkg/version.Version=${VERSION} -X github.com/appvia/terranetes-controller/pkg/version.GitCommit=${GIT_SHA}
VERSION ?= latest

# IMPORTANT NOTE: On CircleCI RELEASE_TAG will be set to the string '<nil>' if no tag is in use, so
# use the local RELEASE variable being 'true' to switch on release build logic.

CLI_PLATFORMS=darwin linux windows
CLI_ARCHITECTURES=amd64 arm64
export GOFLAGS = -mod=vendor

.PHONY: test build docker release check golangci-lint apis images

default: build

golang:
	@echo "--> Go Version"
	@go version
	@echo "GOFLAGS: $$GOFLAGS"
	@mkdir -p bin

### GENERATE ###

apis: golang
	@echo "--> Generating Clientsets & Deepcopies"
	@$(MAKE) controller-gen
	@$(MAKE) register-gen
	@$(MAKE) schema-gen
	@$(MAKE) gofmt

check-apis: apis
	@$(MAKE) check-api-sync

check-api-sync:
	@if [ $$(git status --porcelain | wc -l) -gt 0 ]; then \
		echo "There are local changes after running 'make apis'. Did you forget to run it?"; \
		git status --porcelain; \
		git --no-pager diff ;\
		exit 1; \
	fi

controller-gen:
	@echo "--> Generating deepcopies, CRDs and webhooks"
	@rm -rf charts/terranetes-controller/crds
	@mkdir -p charts/terranetes-controller/crds
	@mkdir -p pkg/client
	@go run sigs.k8s.io/controller-tools/cmd/controller-gen \
		paths=./pkg/apis/... \
		object:headerFile=hack/boilerplate.go.txt \
		crd \
		output:crd:dir=charts/terranetes-controller/crds \
		webhook \
		output:webhook:dir=deploy/webhooks
	@./hack/patch-crd-gen.sh
	@./hack/gofmt.sh pkg/apis/*/*/zz_generated.deepcopy.go

register-gen:
	@echo "--> Generating schema register.go files"
	@$(foreach api,$(APIS), \
		echo "    $(api)" && go run k8s.io/code-generator/cmd/register-gen -h hack/boilerplate.go.txt \
			--output-file-base zz_generated_register \
			-i github.com/appvia/terranetes-controller/pkg/apis/$(api) \
			-p github.com/appvia/terranetes-controller/pkg/apis/$(api); )

schema-gen:
	@echo "--> Generating Kubernetes assets"
	@go run github.com/go-bindata/go-bindata/v3/go-bindata \
    -nocompress \
    -pkg register \
    -nometadata \
    -o pkg/register/assets.go \
    -prefix deploy charts/terranetes-controller/crds deploy/webhooks
	@$(MAKE) gofmt

### BUILD ###

build: controller source step tnctl
	@echo "--> Compiling the project ($(VERSION))"

controller: golang
	@echo "--> Compiling the controller ($(VERSION))"
	CGO_ENABLED=0 go build -ldflags "${LFLAGS}" -tags=jsoniter -o bin/controller cmd/controller/*.go

tnctl: golang
	@echo "--> Compiling the tnctl ($(VERSION))"
	CGO_ENABLED=0 go build -ldflags "${LFLAGS}" -tags=jsoniter -o bin/tnctl cmd/tnctl/*.go

source: golang
	@echo "--> Compiling the source binary ($(VERSION))"
	CGO_ENABLED=0 go build -ldflags "${LFLAGS}" -tags=jsoniter -o bin/source cmd/source/*.go

step: golang
	@echo "--> Compiling the step binary ($(VERSION))"
	CGO_ENABLED=0 go build -ldflags "${LFLAGS}" -tags=jsoniter -o bin/step cmd/step/*.go

### TESTING ###

test:
	@echo "--> Running the tests"
	@rm -f cover.out || true
	@go test -cover -v ./...

### IMAGES ###

cli-image:
	@echo "--> Compiling the cli image ${REGISTRY}/${REGISTRY_ORG}/terranetes-cli:${VERSION}"
	@docker build --build-arg VERSION=${VERSION} -t ${REGISTRY}/${REGISTRY_ORG}/terranetes-cli:${VERSION} -f images/Dockerfile.cli .

cli-image-verify: install-trivy
	@echo "--> Verifying cli server image ${REGISTRY}/${REGISTRY_ORG}/terranetes-cli:${VERSION}"
	echo "--> Checking image ${REGISTRY}/${REGISTRY_ORG}/terranetes-cli:${VERSION} for vulnerabilities"
	PATH=${PATH}:bin/ trivy image --exit-code 1 --severity "CRITICAL" ${REGISTRY}/${REGISTRY_ORG}/terranetes-cli:${VERSION}

controller-image:
	@echo "--> Compiling the terranetes-controller server image ${REGISTRY}/${REGISTRY_ORG}/terranetes-controller:${VERSION}"
	@docker build --build-arg VERSION=${VERSION} -t ${REGISTRY}/${REGISTRY_ORG}/terranetes-controller:${VERSION} -f images/Dockerfile.controller .

controller-kind:
	@echo "--> Updating the kind image for controller and reloading"
	@kubectl -n terraform-system scale deployment terranetes-controller --replicas=0 || true
	@kubectl -n terraform-system delete job --all || true
	@kubectl -n apps delete job --all || true
	@kubectl -n apps delete po --all || true
	@$(MAKE) VERSION=ci controller-image
	@$(MAKE) VERSION=ci executor-image
	@$(MAKE) VERSION=ci controller-kind-images
	@kubectl -n terraform-system scale deployment terranetes-controller --replicas=1 || true

controller-kind-images:
	@echo "--> Loading Controller CI Images into Kind"
	@kind load docker-image ${REGISTRY}/${REGISTRY_ORG}/terranetes-controller:ci
	@kind load docker-image ${REGISTRY}/${REGISTRY_ORG}/terranetes-executor:ci

controller-image-verify: install-trivy
	@echo "--> Verifying controller server image ${REGISTRY}/${REGISTRY_ORG}/terranetes-controller:${VERSION}"
	echo "--> Checking image ${REGISTRY}/${REGISTRY_ORG}/terranetes-controller:${VERSION} for vulnerabilities"
	PATH=${PATH}:bin/ trivy image --exit-code 1 --severity "CRITICAL" ${REGISTRY}/${REGISTRY_ORG}/terranetes-controller:${VERSION}

executor-image:
	@echo "--> Compiling the terranetes-executor server image ${REGISTRY}/${REGISTRY_ORG}/terranetes-executor:${VERSION}"
	@docker build --build-arg VERSION=${VERSION} -t ${REGISTRY}/${REGISTRY_ORG}/terranetes-executor:${VERSION} -f images/Dockerfile.executor .

executor-image-kind: executor-image
	@echo "--> Building and loading executor image ${REGISTRY}/${REGISTRY_ORG}/terranetes-executor:${VERSION}"
	@kind load docker-image ${REGISTRY}/${REGISTRY_ORG}/terranetes-executor:${VERSION}

executor-image-verify: install-trivy
	@echo "--> Verifying executor server image ${REGISTRY}/${REGISTRY_ORG}/terranetes-executor:${VERSION}"
	echo "--> Checking image ${REGISTRY}/${REGISTRY_ORG}/terranetes-executor:${VERSION} for vulnerabilities"
	PATH=${PATH}:bin/ trivy image --exit-code 1 --severity "CRITICAL" ${REGISTRY}/${REGISTRY_ORG}/terranetes-executor:${VERSION}

# Image management

install-trivy:
	@hack/install-trivy.sh

images: controller-image executor-image cli-image
	@echo "--> Building the Images"

verify-images: controller-image-verify executor-image-verify cli-image-verify
	@echo "--> Verifying the Images"

### RELEASE PACKAGING ###

package:
	@rm -rf ./release
	@mkdir ./release
	cd ./release && sha256sum * > terranetes-controller.sha256sums

release-images: images
	@echo "--> Releasing docker images for controller and executor"
	@docker push ${REGISTRY}/${REGISTRY_ORG}/terranetes-controller:${VERSION}
	@docker push ${REGISTRY}/${REGISTRY_ORG}/terranetes-executor:${VERSION}

release-cli: golang
	@echo "--> Compiling static CLI binaries"
	CGO_ENABLED=0 go run github.com/mitchellh/gox -parallel=4 -arch="${CLI_ARCHITECTURES}" -os="${CLI_PLATFORMS}" -ldflags "-w ${LFLAGS}" -output=./release/{{.Dir}}-{{.OS}}-{{.Arch}} ./cmd/tnctl

### CHECKING AND LINTING ###

check-gofmt:
	@echo "--> Running gofmt check"
	@if ./hack/gofmt.sh -l ${GO_DIRS} | grep \.go ; then \
		echo "There are unformatted Go files - did you forget to run 'make gofmt'?"; \
		exit 1; \
	fi

gofmt:
	@echo "--> Running go fmt"
	@go fmt $(shell go list ./... | grep -v /vendor/)
	@echo "--> goimports assets"
	@go run golang.org/x/tools/cmd/goimports -local github.com/appvia/terranetes-controller -w -d $(shell find . -type f -name '*.go' -not -path "./vendor/*")

format: gofmt

bench:
	@echo "--> Running go bench"
	@go test -bench=. -benchmem

coverage:
	@echo "--> Running go coverage"
	@go test -coverprofile cover.out
	@go tool cover -html=cover.out -o cover.html

golangci-lint:
	@echo "--> Checking against the golangci-lint"
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run ./...

lint: golangci-lint

shfmt:
	@echo "--> Running shfmt"
	@go run mvdan.cc/sh/v3/cmd/shfmt -l -w -ci -i 2 -- images/assets

check: test
	@echo "--> Running code checkers"
	@$(MAKE) golang
	@$(MAKE) check-gofmt
	@$(MAKE) shfmt
	@$(MAKE) golangci-lint

### UTILITIES ###

trigger-aws-e2e:
	@echo "--> Triggering the e2e tests on ${GIT_BRANCH} branch (AWS)"
	@gh workflow run e2e.yaml --ref ${GIT_BRANCH} -f cloud=aws -f use_helm=false -f version=ci

trigger-azure-e2e:
	@echo "--> Triggering the e2e tests on ${GIT_BRANCH} branch (Azure)"
	@gh workflow run e2e.yaml --ref ${GIT_BRANCH} -f cloud=azurerm -f use_helm=false -f version=ci

trigger-google-e2e:
	@echo "--> Triggering the e2e tests on ${GIT_BRANCH} branch (Google)"
	@gh workflow run e2e.yaml --ref ${GIT_BRANCH} -f cloud=google -f use_helm=false -f version=ci

trigger-quickstart-e2e:
	@echo "--> Triggering the e2e tests on master using official repository"
	@gh workflow run e2e.yaml --ref master -f cloud=aws -f use_helm=true -f version=na

trigger-all-e2e: trigger-aws-e2e trigger-azure-e2e trigger-google-e2e

diagnostics:
	@echo "--> Retrieving diagnostics"
	@AWS_PROFILE=appvia aws s3 sync s3://terranetes-e2e-diagnostics ./dev/diagnostics/

diagnostics-cleanup:
	@echo "--> Cleaning diagnostics build"
	@AWS_PROFILE=appvia aws s3 rm --recursive s3://terranetes-e2e-diagnostics/github

clean:
	@echo "--> Cleaning up the environment"
	rm -rf ./bin 2>/dev/null
	rm -rf ./release 2>/dev/null

aws-credentials:
	@echo "--> Generating AWS credentials"
	@kubectl create namespace terraform-system 2>/dev/null || true
	@kubectl -n terraform-system create secret generic aws \
		--from-literal=AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} \
		--from-literal=AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} \
		--from-literal=AWS_REGION=${AWS_REGION}

azure-credentials:
	@echo "--> Creating Azure credentials"
	@kubectl create namespace terraform-system 2>/dev/null || true
	@kubectl -n terraform-system create secret generic azurerm \
		--from-literal=ARM_CLIENT_ID=${ARM_CLIENT_ID} \
		--from-literal=ARM_CLIENT_SECRET=${ARM_CLIENT_SECRET} \
		--from-literal=ARM_SUBSCRIPTION_ID=${ARM_SUBSCRIPTION_ID} \
		--from-literal=ARM_TENANT_ID=${ARM_TENANT_ID}

google-credentials:
	@echo "--> Creating Google credentials"
	@kubectl create namespace terraform-system 2>/dev/null || true
	@echo ${GOOGLE_CREDENTIALS} | base64 -d | kubectl -n terraform-system apply -f - 2>/dev/null
