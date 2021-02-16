TEST?=$$(go list ./... | grep -v 'vendor')
HOSTNAME=hashicorp.com
NAMESPACE=qti
NAME=enos
BINARY=terraform-provider-enos
VERSION=0.1
OS_ARCH=darwin_amd64
GLOBAL_BUILD_TAGS=-tags osusergo,netgo
GLOBAL_LD_FLAGS=-ldflags="-extldflags=-static"

default: install

build:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${GLOBAL_BUILD_TAGS} ${GLOBAL_LD_FLAGS} -o ${BINARY}

release:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${GLOBAL_BUILD_TAGS} ${GLOBAL_LD_FLAGS} -o ./bin/${BINARY}_${VERSION}_darwin_amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${GLOBAL_BUILD_TAGS} ${GLOBAL_LD_FLAGS} -o ./bin/${BINARY}_${VERSION}_linux_amd64

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

test:
	go test -i $(TEST) || exit 1
	echo $(TEST) | xargs -t -n4 go test $(TESTARGS) -timeout=30s -parallel=4

testacc:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m
