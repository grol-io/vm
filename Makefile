# all: generate lint check test run

all: clean generate lint test run grol_cvm native

clean:
	rm -f vm grol_cvm tiny_vm a.out

# Use that tags to test the non select cases (wasi, windows,...): test_alt_timeoutreader
# GO_BUILD_TAGS:=no_net,no_pprof,test_alt_timeoutreader
GO_BUILD_TAGS:=no_net,no_pprof

#GROL_FLAGS:=-no-register

run: vm
	./vm compile -loglevel debug programs/simple.asm
	od -t x8 programs/simple.vm
	./vm run -loglevel debug programs/simple.vm
	./vm compile -loglevel debug programs/addr.asm
	./vm run -loglevel debug programs/addr.vm
	./vm compile -loglevel debug programs/hello.asm
	od -a programs/hello.vm
	./vm run -loglevel debug programs/hello.vm
	./vm compile -loglevel debug programs/loop.asm
	time ./vm run -profile-cpu cpu.pprof programs/loop.vm

GEN:=cpu/instruction_string.go cpu/syscall_string.go

vm: Makefile *.go */*.go $(GEN)
#	CGO_ENABLED=0 go build -trimpath -ldflags="-s" -tags "$(GO_BUILD_TAGS)" .
	CGO_ENABLED=0 go build .
	ls -lh vm

CC:=gcc

grol_cvm: Makefile cvm/cvm.c
	$(CC) -O3 -Wall -Wextra -pedantic -Werror -o grol_cvm cvm/cvm.c
	time ./grol_cvm programs/loop.vm
	./grol_cvm programs/hello.vm

debug-cvm:
	$(CC) -O3 -Wall -Wextra -pedantic -Werror -DDEBUG=1 -o grol_cvm cvm/cvm.c
	./grol_cvm programs/simple.vm
	./grol_cvm programs/addr.vm

native: Makefile cvm/loop.c
	$(CC) -O3 -Wall -Wextra -pedantic -Werror cvm/loop.c
	time ./a.out programs/loop.vm

TINY_OPTS:=-opt 2
tiny_vm: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 tinygo build -o tiny_vm $(TINY_OPTS) .
	time ./tiny_vm run programs/loop.vm

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

cpu/instruction_string.go: cpu/instruction.go
	go generate ./cpu # if this fails go install golang.org/x/tools/cmd/stringer@latest

cpu/syscall_string.go: cpu/syscall.go
	go generate ./cpu # if this fails go install golang.org/x/tools/cmd/stringer@latest

.PHONY: all lint generate test clean run build vm install unit-tests
.PHONY: show_cpu_profile show_mem_profile native debug_cvm

show_cpu_profile:
	-pkill pprof
	go tool pprof -http :8080 ./vm cpu.pprof

show_mem_profile:
	-pkill pprof
	go tool pprof -http :8081 ./vm mem.pprof
