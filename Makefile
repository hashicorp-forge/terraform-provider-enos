TEST?=$$(go list ./... | grep -v 'vendor')
HOSTNAME=hashicorp.com
NAMESPACE=qti
NAME=enos
BINARY=terraform-provider-enos
BIN_OS=$$(go env GOOS)
BIN_ARCH=$$(go env GOARCH)
VERSION=$$(cat VERSION)
GLOBAL_BUILD_TAGS=-tags osusergo,netgo
GLOBAL_LD_FLAGS=-ldflags="-extldflags=-static"
FLIGHTCONTROL_LD_FLAGS=-ldflags="-extldflags=-static -s -w"
CI?=false
# Under no circumstances should you use a newer version.
# https://github.com/goreleaser/goreleaser/pull/3213 broke our multiple build
# logic, thus it creates binaries without flight-control embedded.
GO_RELEASER_DOCKER_TAG=v1.9.2
HASUPX:= $(shell upx dot 2> /dev/null)
TEST_BLD_DIR=./test-build
ENOS_CLI_TEST_DIR=$(TEST_BLD_DIR)/enoscli-tests
ENOS_RELEASE_NAME?=enos

# Heavy sigh, sed uses slightly different syntax on linux than macos, here we setup the opts assuming
# CI=true is linux and CI=false is macos
SED_OPTS=-i ''
ifeq ($(CI), true)
SED_OPTS=-i -e
endif

default: install

build:
	CGO_ENABLED=0 go build ${GLOBAL_BUILD_TAGS} ${GLOBAL_LD_FLAGS} -gcflags="all=-N -l" -o ${BINARY} ./command/plugin

release:
ifeq ($(CI), true)
	docker run --rm --privileged --env VERSION=${VERSION} \
		-v $(shell pwd):/go/src/github.com/hashicorp/enos-provider \
		-w /go/src/github.com/hashicorp/enos-provider goreleaser/goreleaser:${GO_RELEASER_DOCKER_TAG} build \
		--rm-dist --snapshot \
		--config build.goreleaser.yml
else
	CGO_ENABLED=0 go build ${GLOBAL_BUILD_TAGS} ${GLOBAL_LD_FLAGS} -o ./dist/${BINARY}_${VERSION}_${BIN_OS}_${BIN_ARCH} ./command/plugin

	echo ${BINARY}_${VERSION}_$(BIN_OS)_$(BIN_ARCH)
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$(BIN_OS)_$(BIN_ARCH)
	cp ./dist/${BINARY}_${VERSION}_$(BIN_OS)_$(BIN_ARCH) ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$(BIN_OS)_$(BIN_ARCH)/
endif

install: release
ifeq ($(CI), true)
	for os_arch in $$(ls -la ./dist | grep ${BINARY} | cut -f 2-3 -d '_') ; do \
		mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$$os_arch ; \
		cp ./dist/${BINARY}_$$os_arch*/${BINARY}_${VERSION} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$$os_arch/${BINARY} ; \
	done
endif

install-race-detector:
	go build -race ${GLOBAL_BUILD_TAGS} ${GLOBAL_LD_FLAGS} -o ./dist/${BINARY}_${VERSION}_${BIN_OS}_${BIN_ARCH} ./command/plugin

	echo ${BINARY}_${VERSION}_$(BIN_OS)_$(BIN_ARCH)
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$(BIN_OS)_$(BIN_ARCH)
	cp ./dist/${BINARY}_${VERSION}_$(BIN_OS)_$(BIN_ARCH) ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$(BIN_OS)_$(BIN_ARCH)/

flight-control: flight-control-build flight-control-pack

flight-control-build:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${GLOBAL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_darwin_amd64 ./command/enos-flight-control
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${GLOBAL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_linux_amd64 ./command/enos-flight-control
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ${GLOBAL_BUILD_TAGS} ${FLIGHTCONTROL_LD_FLAGS} -o internal/flightcontrol/binaries/enos-flight-control_linux_arm64 ./command/enos-flight-control

flight-control-pack:
ifndef HASUPX
	$(error "upx is required to pack enos-flight-control - get it via `brew install upx`")
endif
	pushd ./internal/flightcontrol/binaries || exit 1; \
	upx -q -9 *; \
	popd || exit 1 \

test:
	go test $(TEST) -v $(TESTARGS) -timeout=5m -parallel=4

# test-tf requires terraform 0.15.0 or higher
test-tf: install
	terraform -chdir=examples/core init
	terraform -chdir=examples/core fmt -check -recursive
	terraform -chdir=examples/core validate

test-acc:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

test-race-detector:
	GORACE=log_path=/tmp/gorace.log TF_ACC=1 go test -race $(TEST) -v $(TESTARGS) -timeout 120m ./command/plugin

# run the k8s enoscli tests for the stable release
test-k8s:
	rm -rf $(ENOS_CLI_TEST_DIR); mkdir -p $(ENOS_CLI_TEST_DIR); \
	cp -r enoscli-tests $(TEST_BLD_DIR); \
    LC_ALL=C grep -lr "TFC_API_TOKEN" $(ENOS_CLI_TEST_DIR) | xargs sed $(SED_OPTS) "s/TFC_API_TOKEN/$(TFC_API_TOKEN)/g" ; \
    LC_ALL=C grep -lr "ENOS_RELEASE_NAME" $(ENOS_CLI_TEST_DIR) | xargs sed $(SED_OPTS) "s/ENOS_RELEASE_NAME/$(ENOS_RELEASE_NAME)/g" ; \
    enos scenario launch -d $(ENOS_CLI_TEST_DIR) kind_cluster; \
    enos scenario output -d $(ENOS_CLI_TEST_DIR) kind_cluster; \
    enos scenario destroy -d $(ENOS_CLI_TEST_DIR) kind_cluster

# Sets the enos release name for testing the dev release
set-dev-release-name:
	export ENOS_RELEASE_NAME="enosdev"

# run the k8s enoscli tests for the dev release
test-k8s-dev: set-dev-release-name test-k8s

lint:
	golangci-lint run -v

fmt:
	gofumpt -w -l .

clean:
	rm -rf dist bin .terraform*
