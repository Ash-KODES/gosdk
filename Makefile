ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

GOMODCORE           := $(GOMODBASE)/zcncore
VERSION_FILE        := $(ROOT_DIR)/core/version/version.go
MAJOR_VERSION       := "1.0"

PLATFORM:=$(shell uname -s | tr "[:upper:]" "[:lower:]")

include _util/printer.mk
include _util/build_$(PLATFORM).mk
include _util/build_mobile.mk

.PHONY: build-tools install-all herumi-all gosdk-all sdkver help lint

default: help

#GO BUILD SDK
gomod-download:
	go mod download -json

gomod-clean:
	go clean -i -r -x -modcache  ./...

clean-gosdk:

gosdk-build: gomod-download
	go build -x -v -tags bn256 ./...

wasm-build:
	CGO_ENABLED=0 GOOS=js GOARCH=wasm go build -o sdk.wasm github.com/0chain/gosdk/wasmsdk
     
wasm-test:
	env -i $(shell go env) PATH="$(shell go env GOROOT)/misc/wasm:$(PATH)" CGO_ENABLED=0 GOOS=js GOARCH=wasm go test -v github.com/0chain/gosdk/wasmsdk/jsbridge/...

gosdk-mocks:
	./generate_mocks.sh

gosdk-test:
	go test -v -tags bn256 ./...

install-gosdk: | gosdk-build wasm-build

$(GOPATH)/bin/modvendor:
	@go get -u github.com/goware/modvendor

vendor: $(GOPATH)/bin/modvendor
	@GO111MODULE=on go mod vendor -v
	@modvendor -copy="**/*.c **/*.h **/*.a" -v

getrev:
	$(eval VERSION_STR=$(shell git describe --tags --dirty --always))
	@echo "" > $(VERSION_FILE)
	@echo "//====== THIS IS AUTOGENERATED FILE. DO NOT MODIFY ========" >> $(VERSION_FILE)
	@echo "" >> $(VERSION_FILE)
	@echo "package version" >> $(VERSION_FILE)
	@echo const VERSIONSTR = \"$(VERSION_STR)\" >> $(VERSION_FILE)
	@echo "" >> $(VERSION_FILE)

install: install-gosdk sdkver

test: gosdk-test

clean: clean-gosdk clean-herumi
	@rm -rf $(OUTDIR)

lint-wasm:
	GOOS=js GOARCH=wasm CGO_ENABLED=0 golangci-lint run wasmsdk --disable-all -E errcheck

lint: lint-wasm
	golangci-lint run --skip-dirs wasmsdk

help:
	@echo "Environment: "
	@echo "\tPLATFORM.......: $(PLATFORM)"
	@echo "\tGOPATH.........: $(GOPATH)"
	@echo "\tGOROOT.........: $(GOROOT)"
	@echo ""
	@echo "Supported commands:"
	@echo "\tmake help              - Display environment and make targets"
	@echo "\tmake build-tools       - Install go, jq and supporting tools required for build"
	@echo "\tmake install           - Install gosdk"
	@echo "\tmake clean             - Deletes all build output files"
	@echo "\tmake lint              - Runs the golangci-lint

install-herumi-ubuntu:
	@cd /tmp && \
        wget -O - https://github.com/herumi/mcl/archive/master.tar.gz | tar xz && \
        wget -O - https://github.com/herumi/bls/archive/master.tar.gz | tar xz && \
        rm -rf mcl && mv mcl* mcl && \
        rm -rf bls &&mv bls* bls && \
        make -C mcl -j $(nproc) lib/libmclbn256.so install && \
        cp mcl/lib/libmclbn256.so /usr/local/lib && \
        make MCL_DIR=/tmp/mcl -C bls -j $(nproc) install && \
        rm -R /tmp/mcl && \
        rm -R /tmp/bls
