# all: generate lint check test run

all: generate lint test run grol_cvm

# Use that tags to test the non select cases (wasi, windows,...): test_alt_timeoutreader
# GO_BUILD_TAGS:=no_net,no_pprof,test_alt_timeoutreader
GO_BUILD_TAGS:=no_net,no_pprof

#GROL_FLAGS:=-no-register

run: vm
	./vm compile -loglevel debug programs/simple.asm
	od -t x1 programs/simple.vm
	./vm run -loglevel debug programs/simple.vm
	./vm compile -loglevel debug programs/loop.asm
	time ./vm run -profile-cpu cpu.pprof programs/loop.vm

GEN:=cpu/instruction_string.go

vm: Makefile *.go */*.go $(GEN)
#	CGO_ENABLED=0 go build -trimpath -ldflags="-s" -tags "$(GO_BUILD_TAGS)" .
	CGO_ENABLED=0 go build .
	ls -lh vm

CC:=gcc-15

grol_cvm: Makefile cvm/cvm.c
	$(CC) -O3 -Wall -Wextra -pedantic -Werror -o grol_cvm cvm/cvm.c
	time ./grol_cvm programs/loop.vm

vm-debug: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 go build -tags debug -o vm-debug .

run-debug: vm-debug
	./vm-debug run -loglevel debug programs/simple.vm

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


generate: $(GEN)

cpu/instruction_string.go: cpu/cpu.go
	go generate ./cpu # if this fails go install golang.org/x/tools/cmd/stringer@latest

.PHONY: all lint generate test clean run build vm install unit-tests
.PHONY: show_cpu_profile show_mem_profile

show_cpu_profile:
	-pkill pprof
	go tool pprof -http :8080 ./vm cpu.pprof

show_mem_profile:
	-pkill pprof
	go tool pprof -http :8081 ./vm mem.pprof
