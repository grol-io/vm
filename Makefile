# all: generate lint check test run

all: lint run

# Use that tags to test the non select cases (wasi, windows,...): test_alt_timeoutreader
# GO_BUILD_TAGS:=no_net,no_pprof,test_alt_timeoutreader
GO_BUILD_TAGS:=no_net,no_pprof

#GROL_FLAGS:=-no-register

run: vm
	# Interactive debug run: use logger with file and line numbers
	LOGGER_IGNORE_CLI_MODE=true GOMEMLIMIT=1GiB ./vm -loglevel debug $(GROL_FLAGS)

GEN:=

vm: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 go build -trimpath -ldflags="-s" -tags "$(GO_BUILD_TAGS)" .
	ls -lh vm


install:
	CGO_ENABLED=0 go install -trimpath -ldflags="-s" -tags "$(GO_BUILD_TAGS)" grol.io/vm@$(GIT_TAG)
	ls -lh "$(shell go env GOPATH)/bin/vm"
	vm version


test: vm unit-tests

unit-tests:
	CGO_ENABLED=0 go test -tags $(GO_BUILD_TAGS) ./...

lint: .golangci.yml
	CGO_ENABLED=0 golangci-lint run --build-tags $(GO_BUILD_TAGS)

.golangci.yml: Makefile
	curl -fsS -o .golangci.yml https://raw.githubusercontent.com/fortio/workflows/main/golangci.yml

.PHONY: all lint generate test clean run build vm install unit-tests
