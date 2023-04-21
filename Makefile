GO_VERSION=$$(cat .go-version)

PROVIDER_HOSTNAME=app.terraform.io
PROVIDER_NAMESPACE=hashicorp-qti
PROVIDER_NAME?=enos
PROVIDER_BIN_NAME=terraform-provider-enos
PROVIDER_BIN_OS?=$$(go env GOOS)
PROVIDER_BIN_ARCH?=$$(go env GOARCH)
PROVIDER_BIN_VERSION=$$(cat VERSION)
PROVIDER_BUILD_TAGS?=-tags osusergo,netgo
PROVIDER_LD_FLAGS?=-ldflags="-extldflags=-static"

FLIGHTCONTROL_BUILD_TAGS?=-tags osusergo,netgo
FLIGHTCONTROL_LD_FLAGS?=-ldflags="-extldflags=-static -s -w"

CI?=false
ENOS_CLI_TEST_DIR=$(TEST_BLD_DIR)/enoscli-tests
LINT_OUT_FORMAT?=colored-line-number
HASUPX:= $(shell upx dot 2> /dev/null)
TEST?=$$(go list ./... | grep -v 'vendor')
TEST_BLD_DIR=./test-build

# Heavy sigh, sed uses slightly different syntax on linux than macos, here we setup the opts assuming
# CI=true is linux and CI=false is macos
SED_OPTS=-i ''
ifeq ($(CI), true)
SED_OPTS=-i -e
endif

# Make sure our shell isn't set to /bin/sh because we use pushd/popd
SHELL = /bin/bash

default: build install

build:
	mkdir -p ./dist
	CGO_ENABLED=0 GOOS=${PROVIDER_BIN_OS} GOARCH=${PROVIDER_BIN_ARCH} go build ${PROVIDER_BUILD_TAGS} ${PROVIDER_LD_FLAGS} -o ./dist/${PROVIDER_BIN_NAME}_${PROVIDER_BIN_VERSION}_${PROVIDER_BIN_OS}_${PROVIDER_BIN_ARCH} ./command/plugin

build-race-detector:
	CGO_ENABLED=0 GOOS=${PROVIDER_BIN_OS} GOARCH=${PROVIDER_BIN_ARCH} go build -race ${PROVIDER_BUILD_TAGS} ${PROVIDER_LD_FLAGS} -o ./dist/${PROVIDER_BIN_NAME}_${PROVIDER_BIN_VERSION}_${PROVIDER_BIN_OS}_${PROVIDER_BIN_ARCH} ./command/plugin

build-all: flight-control build

install:
	for binary in $$(ls ./dist | grep ${PROVIDER_BIN_NAME}) ; do \
	version=$$(echo $$binary | cut -d "_" -f 2); \
	platform=$$(echo $$binary | cut -d "_" -f 3); \
	arch=$$(echo $$binary | cut -d "_" -f 4); \
	mkdir -p ~/.terraform.d/plugins/${PROVIDER_HOSTNAME}/${PROVIDER_NAMESPACE}/${PROVIDER_NAME}/$${version}/$${platform}_$${arch}; \
	cp ./dist/$$binary ~/.terraform.d/plugins/${PROVIDER_HOSTNAME}/${PROVIDER_NAMESPACE}/${PROVIDER_NAME}/$${version}/$${platform}_$${arch}/${PROVIDER_BIN_NAME}; \
done

flight-control: flight-control-build flight-control-pack

flight-control-build:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${FLIGHTCONTROL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_darwin_amd64 ./command/enos-flight-control
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${FLIGHTCONTROL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_linux_amd64 ./command/enos-flight-control
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ${FLIGHTCONTROL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_linux_arm64 ./command/enos-flight-control

flight-control-pack:
ifndef HASUPX
	$(error "upx is required to pack enos-flight-control - get it via `brew install upx`")
endif
	pushd ./internal/flightcontrol/binaries || exit 1; \
	upx -q -9 *; \
	popd || exit 1 \

test:
	go test $(TEST) -v $(TESTARGS) -timeout=5m -parallel=4

test-tf:
	terraform -chdir=examples/core init
	terraform -chdir=examples/core fmt -check -recursive
	terraform -chdir=examples/core validate

test-acc:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 12m

test-race-detector:
	GORACE=log_path=/tmp/gorace.log TF_ACC=1 go test -race $(TEST) -v $(TESTARGS) -timeout 120m ./command/plugin

# run the k8s enoscli tests for the stable release
.PHONY: test-k8s
test-k8s: K8S_TEST_DIR = $(ENOS_CLI_TEST_DIR)/k8s
test-k8s:
	rm -rf $(ENOS_CLI_TEST_DIR); mkdir -p $(ENOS_CLI_TEST_DIR); \
	cp -r enoscli-tests $(TEST_BLD_DIR); \
    LC_ALL=C grep -lr "ENOS_PROVIDER_NAME" $(ENOS_CLI_TEST_DIR) | xargs sed $(SED_OPTS) "s/ENOS_PROVIDER_NAME/$(PROVIDER_NAME)/g" ; \
    LC_ALL=C grep -lr "ENOS_PROVIDER_VERSION" $(ENOS_CLI_TEST_DIR) | xargs sed $(SED_OPTS) "s/ENOS_PROVIDER_VERSION/$(PROVIDER_BIN_VERSION)/g" ; \
    enos scenario launch -d $(K8S_TEST_DIR) kind_cluster; \
    enos scenario output -d $(K8S_TEST_DIR) kind_cluster; \
    enos scenario destroy -d $(K8S_TEST_DIR) kind_cluster

# run the k8s enoscli tests for the dev release
.PHONY: test-k8s-dev
test-k8s-dev: PROVIDER_NAME = enosdev
test-k8s-dev: test-k8s

.PHONY: fmt
fmt: fmt-golang fmt-enos

.PHONY: fmt-golang
fmt-golang:
	gofumpt -w -l .

.PHONY: fmt-enos
fmt-enos:
	enos fmt enos
	enos fmt enoscli-tests
	terraform fmt -recursive enos
	terraform fmt -recursive enoscli-tests
	terraform fmt -recursive examples
	terraform fmt -recursive ci-tests

.PHONY: fmt-check
fmt-check: fmt-check-golang fmt-check-enos

.PHONY: fmt-check-golang
fmt-check-golang:
	gofumpt -d -l .

.PHONY: fmt-check-enos
fmt-check-enos:
	enos fmt -cd enos
	enos fmt -cd enoscli-tests
	terraform fmt -recursive -check enos
	terraform fmt -recursive -check enoscli-tests
	terraform fmt -recursive -check examples
	terraform fmt -recursive -check ci-tests

.PHONY: lint
lint:
	golangci-lint run -v --out-format=$(LINT_OUT_FORMAT)

.PHONY: clean
clean:
	rm -rf dist bin .terraform*
