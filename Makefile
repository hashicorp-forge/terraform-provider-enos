THIS_FILE := $(lastword $(MAKEFILE_LIST))
GO_VERSION=$$(cat .go-version)

PROVIDER_HOSTNAME=registry.terraform.io
PROVIDER_NAMESPACE=hashicorp-forge
PROVIDER_NAME?=enos
PROVIDER_BIN_NAME=terraform-provider-enos
PROVIDER_BIN_OS?=$$(go env GOOS)
PROVIDER_BIN_ARCH?=$$(go env GOARCH)
PROVIDER_BIN_VERSION?=$$(cat VERSION)
PROVIDER_BUILD_TAGS?=-tags osusergo,netgo
PROVIDER_LD_FLAGS?=-ldflags="-extldflags=-static"

FLIGHTCONTROL_BUILD_TAGS?=-tags osusergo,netgo
FLIGHTCONTROL_LD_FLAGS?=-ldflags="-extldflags=-static -s -w"

CI?=false
LINT_OUT_FORMAT?=colored-line-number
.PHONY: HASUPX
HASUPX:= $(shell upx dot 2> /dev/null)
BUILD_DARWIN_FC?=false
TEST?=./...
TEST_BLD_DIR=./test-build

# Make sure our shell isn't set to /bin/sh because we use pushd/popd
SHELL = /bin/bash

.PHONY: default
default: build install

.PHONY: all
all: clean lint fmt-check build-all install test-race-detector

.PHONY: build
build:
	mkdir -p ./dist
	CGO_ENABLED=0 GOOS=${PROVIDER_BIN_OS} GOARCH=${PROVIDER_BIN_ARCH} go build ${PROVIDER_BUILD_TAGS} ${PROVIDER_LD_FLAGS} -o ./dist/${PROVIDER_BIN_NAME}_${PROVIDER_BIN_VERSION}_${PROVIDER_BIN_OS}_${PROVIDER_BIN_ARCH} ./command/plugin

.PHONY: build-race-detector
build-race-detector:
	CGO_ENABLED=0 GOOS=${PROVIDER_BIN_OS} GOARCH=${PROVIDER_BIN_ARCH} go build -race ${PROVIDER_BUILD_TAGS} ${PROVIDER_LD_FLAGS} -o ./dist/${PROVIDER_BIN_NAME}_${PROVIDER_BIN_VERSION}_${PROVIDER_BIN_OS}_${PROVIDER_BIN_ARCH} ./command/plugin

.PHONY: build-all
build-all: flight-control build

.PHONY: docs
docs:
	type tfplugindocs || go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	tfplugindocs generate --examples-dir examples --provider-dir command/plugin --provider-name ${PROVIDER_BIN_NAME} --rendered-website-dir ../../docs --website-source-dir ./templates

.PHONY: check-doc-delta
check-doc-delta:
	rm -rf ./docs
	@$(MAKE) -f $(THIS_FILE) docs
	@if ! git diff --exit-code; then echo "Documentation need to be regenerated. Run 'make docs' to fix them." && exit 1; fi

install:
	for binary in $$(ls ./dist | grep ${PROVIDER_BIN_NAME}) ; do \
	version=$$(echo $$binary | cut -d "_" -f 2); \
	platform=$$(echo $$binary | cut -d "_" -f 3); \
	arch=$$(echo $$binary | cut -d "_" -f 4); \
	mkdir -p ~/.terraform.d/plugins/${PROVIDER_HOSTNAME}/${PROVIDER_NAMESPACE}/${PROVIDER_NAME}/$${version}/$${platform}_$${arch}; \
	cp ./dist/$$binary ~/.terraform.d/plugins/${PROVIDER_HOSTNAME}/${PROVIDER_NAMESPACE}/${PROVIDER_NAME}/$${version}/$${platform}_$${arch}/${PROVIDER_BIN_NAME}; \
	chmod +x ~/.terraform.d/plugins/${PROVIDER_HOSTNAME}/${PROVIDER_NAMESPACE}/${PROVIDER_NAME}/$${version}/$${platform}_$${arch}/*; \
done

.PHONY: flight-control
flight-control: flight-control-build flight-control-pack

.PHONY: flight-control-build
flight-control-build:
ifeq ($(BUILD_DARWIN_FC), true)
	# Don't build the Darwin flight-control binaries by default since we cannot pack them and we don't
	# actually test against macOS targets right now. Building these will also cause some tests to fail
	# if you don't remove them from internal/flight-control/binaries.
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${FLIGHTCONTROL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_darwin_amd64 ./command/enos-flight-control
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ${FLIGHTCONTROL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_darwin_amd64 ./command/enos-flight-control
endif
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${FLIGHTCONTROL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_linux_amd64 ./command/enos-flight-control
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ${FLIGHTCONTROL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_linux_arm64 ./command/enos-flight-control
	CGO_ENABLED=0 GOOS=linux GOARCH=s390x go build ${FLIGHTCONTROL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_linux_s390x ./command/enos-flight-control

.PHONY: flight-control-pack
flight-control-pack:
ifndef HASUPX
	@echo "upx was not found, enos-flight-control binaries will not be packed"
	exit 0
endif
	# We also can't currently safely pack macOS for now, see https://github.com/upx/upx/issues/612
	upx -q -9 \
    ./internal/flightcontrol/binaries/enos-flight-control_linux_amd64 \
    ./internal/flightcontrol/binaries/enos-flight-control_linux_arm64;

.PHONY: test
test:
	go test $(TEST) -v $(TESTARGS) -timeout=5m -parallel=4

.PHONY: test-acc
test-acc:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 12m

.PHONY: test-race-detector
test-race-detector:
	GORACE=log_path=/tmp/gorace.log TF_ACC=1 go test -race $(TEST) -v $(TESTARGS) -timeout 120m ./command/plugin

.PHONY: fmt
fmt: fmt-golang fmt-enos

.PHONY: fmt-golang
fmt-golang:
	gofumpt -w -l .

.PHONY: fmt-enos
fmt-enos:
	enos fmt enos
	terraform fmt -recursive enos
	terraform fmt -recursive examples

.PHONY: fmt-check
fmt-check: fmt-check-golang fmt-check-enos

.PHONY: fmt-check-golang
fmt-check-golang:
	gofumpt -d -l .

.PHONY: fmt-check-enos
fmt-check-enos:
	enos fmt -cd enos
	terraform fmt -recursive -check enos
	terraform fmt -recursive -check examples

.PHONY: lint
lint:
	golangci-lint run -v --out-format=$(LINT_OUT_FORMAT)

.PHONY: lint-fix
lint-fix:
	golangci-lint run -v --out-format=$(LINT_OUT_FORMAT) --fix

.PHONY: clean
clean:
	rm -rf dist/* bin/* .terraform*
