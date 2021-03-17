TEST?=$$(go list ./... | grep -v 'vendor')
HOSTNAME=hashicorp.com
NAMESPACE=qti
NAME=enos
BINARY=terraform-provider-enos
BIN_OS=$$(go env GOOS)
BIN_ARCH=$$(go env GOARCH)
VERSION=0.1
GLOBAL_BUILD_TAGS=-tags osusergo,netgo
GLOBAL_LD_FLAGS=-ldflags="-extldflags=-static"
CI?=false
GO_RELEASER_DOCKER_TAG=v0.159.0 # "latest" is not actually the latest

default: install

build:
	CGO_ENABLED=0 go build ${GLOBAL_BUILD_TAGS} ${GLOBAL_LD_FLAGS} -o ${BINARY}

release:
ifeq ($(CI), true)
	docker run --rm --privileged --env VERSION=${VERSION} \
		-v $(shell pwd):/go/src/github.com/user/repo \
		-w /go/src/github.com/user/repo goreleaser/goreleaser:${GO_RELEASER_DOCKER_TAG} build \
		--rm-dist --snapshot \
		--config build.goreleaser.yml
else
	CGO_ENABLED=0 go build ${GLOBAL_BUILD_TAGS} ${GLOBAL_LD_FLAGS} -o ./dist/${BINARY}_${VERSION}_${BIN_OS}_${BIN_ARCH}

	echo ${BINARY}_${VERSION}_$(BIN_OS)_$(BIN_ARCH)
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$(BIN_OS)_$(BIN_ARCH)
	cp ./dist/${BINARY}_${VERSION}_$(BIN_OS)_$(BIN_ARCH) ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$(BIN_OS)_$(BIN_ARCH)/
endif

install: release
ifeq ($(CI), true)
	for os_arch in $$(ls -la ./dist | grep ${BINARY} | cut -f 2-3 -d '_') ; do \
		mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$$os_arch ; \
		cp ./dist/${BINARY}_$$os_arch/${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$$os_arch/ ; \
	done
endif

test:
	go test -vi $(TEST) || exit 1
	echo $(TEST) | xargs -t -n4 go test -v $(TESTARGS) -timeout=30s -parallel=4

tftest: install
	terraform init examples/core
	terraform fmt -check -recursive examples/core
	terraform validate examples/core

testacc:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

lint:
	golangci-lint run -v

clean:
	rm -rf dist bin .terraform*
