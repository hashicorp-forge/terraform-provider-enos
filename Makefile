TEST?=$$(go list ./... | grep -v 'vendor')
HOSTNAME=hashicorp.com
NAMESPACE=qti
NAME=enos
BINARY=terraform-provider-enos
VERSION=0.1
GLOBAL_BUILD_TAGS=-tags osusergo,netgo
GLOBAL_LD_FLAGS=-ldflags="-extldflags=-static"
CI?=false

default: install

build:
	CGO_ENABLED=0 go build ${GLOBAL_BUILD_TAGS} ${GLOBAL_LD_FLAGS} -o ${BINARY}

release:
ifeq ($(CI), true)
	docker run --rm --privileged --env VERSION=${VERSION} \
		-v $(shell pwd):/go/src/github.com/user/repo \
		-w /go/src/github.com/user/repo goreleaser/goreleaser build \
		--rm-dist --snapshot \
		--config build.goreleaser.yml
else
	@echo "We only run releases through CI, please merge or *danger* set CI='true' *danger*"
endif

install: release
	for os_arch in $$(ls -la ./dist | grep ${BINARY} | cut -f 2-3 -d '_') ; do \
		mkdir -p .terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$$os_arch ; \
		cp ./dist/${BINARY}_$$os_arch/${BINARY} .terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/$$os_arch/ ; \
	done

test:
	go test -vi $(TEST) || exit 1
	echo $(TEST) | xargs -t -n4 go test -v $(TESTARGS) -timeout=30s -parallel=4

tftest: install
	terraform init -plugin-dir .terraform.d/plugins internal/config
	terraform fmt -check -recursive internal/config
	terraform validate internal/config

testacc:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

lint:
	golangci-lint run -v

clean:
	rm -rf dist bin .terraform*