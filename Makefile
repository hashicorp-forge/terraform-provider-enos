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
GO_RELEASER_DOCKER_TAG=latest
HASUPX:= $(shell upx dot 2> /dev/null)

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
	upx --ultra-brute *; \
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

lint:
	golangci-lint run -v

fmt:
	gofumpt -w -l .

clean:
	rm -rf dist bin .terraform*
