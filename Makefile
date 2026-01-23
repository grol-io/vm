# all: generate lint check test run

all: clean generate lint run test

test: vm unit-tests itoa-test fact cat-test wc-test echo-test timing-tests

timing-tests: timing-loop cvm-loop nativecloop elf64

clean:
	rm -f vm grol_cvm tiny_vm a.out native/sample/loop

# Use that tags to test the non select cases (wasi, windows,...): test_alt_timeoutreader
# GO_BUILD_TAGS:=no_net,no_pprof,test_alt_timeoutreader
GO_BUILD_TAGS:=no_net,no_pprof

#GROL_FLAGS:=-no-register

itoa-test: vm grol_cvm
	./vm compile programs/itoa.asm
	./vm run -quiet programs/itoa.vm
	./grol_cvm programs/itoa.vm

SAMPLE_CAT:=cpu/cpu.go

cat-test: vm grol_cvm
	./vm compile programs/cat.asm
	./vm run -quiet programs/cat.vm < $(SAMPLE_CAT) > /tmp/cat_output
	cmp $(SAMPLE_CAT) /tmp/cat_output
	./grol_cvm programs/cat.vm < $(SAMPLE_CAT) > /tmp/cat_output
	cmp $(SAMPLE_CAT) /tmp/cat_output
	/bin/echo -n "A" > /tmp/cat_input
	./vm run -quiet programs/cat.vm < /tmp/cat_input > /tmp/cat_single
	cmp /tmp/cat_input /tmp/cat_single
	./grol_cvm programs/cat.vm < /tmp/cat_input > /tmp/cat_single
	cmp /tmp/cat_input /tmp/cat_single

wc-test: vm grol_cvm
	./vm compile programs/wc.asm programs/itoa.asm
	./vm run -quiet programs/wc.vm < $(SAMPLE_CAT) > /tmp/wc_output
	wc -l < $(SAMPLE_CAT) | awk '{print $$1}' > /tmp/wc_expected
	diff /tmp/wc_expected /tmp/wc_output
	./grol_cvm programs/wc.vm < $(SAMPLE_CAT) > /tmp/wc_output
	od -c /tmp/wc_expected
	od -c /tmp/wc_output
	diff /tmp/wc_expected /tmp/wc_output
	./vm run -quiet programs/wc.vm programs/*.asm > /tmp/wc_all_output
	wc -l programs/*.asm | awk '/total/ {print("total:", $$1)} !/total/{print($$2, $$1)}' > /tmp/wc_all_expected
	diff /tmp/wc_all_expected /tmp/wc_all_output
	./grol_cvm programs/wc.vm programs/*.asm > /tmp/wc_all_output
	diff /tmp/wc_all_expected /tmp/wc_all_output
	# error case --- file doesn't exist will abort with error message on stderr
	./vm run -loglevel critical programs/wc.vm programs/wc.asm nofilesuchfile programs/simple.asm; test $$? -eq 1
	./grol_cvm programs/wc.vm programs/wc.asm nofilesuchfile programs/simple.asm; test $$? -eq 1

echo-test: vm grol_cvm
	./vm compile programs/echo.asm
	./vm run -quiet programs/echo.vm A B "" "4th argument (after empty one) a bit longer"
	./grol_cvm programs/echo.vm A B "" "4th argument (after empty one) a bit longer"

run: vm
	./vm compile -loglevel debug programs/simple.asm
	od -t x8 programs/simple.vm
	./vm run -loglevel debug programs/simple.vm
	./vm compile -loglevel debug programs/addr.asm
	./vm run -loglevel debug programs/addr.vm
	./vm compile -loglevel debug programs/hello.asm
	od -a programs/hello.vm
	./vm run -loglevel debug programs/hello.vm
	./vm compile -loglevel debug programs/itoa.asm
	./vm run -quiet programs/itoa.vm
	./vm compile -loglevel debug programs/rune_literal.asm
	./vm run -loglevel debug programs/rune_literal.vm
	./vm compile -loglevel debug programs/incr.asm
	./vm run -loglevel debug programs/incr.vm
	./vm compile -loglevel debug programs/pow.asm
	./vm run -loglevel debug programs/pow.vm
	./vm compile programs/compare_neg.asm programs/itoa.asm
	./vm run -quiet programs/compare_neg.vm

timing-loop: vm
	./vm compile -loglevel debug programs/loop.asm
	time ./vm run -profile-cpu cpu.pprof programs/loop.vm # with profiler on
	time ./vm run -quiet programs/loop.vm # without

GEN:=cpu/instruction_string.go cpu/syscall_string.go

vm: Makefile *.go */*.go $(GEN)
#	CGO_ENABLED=0 go build -trimpath -ldflags="-s" -tags "$(GO_BUILD_TAGS)" .
	CGO_ENABLED=0 go build .
	ls -lh vm

CC:=gcc

cvm/cvm.h: vm asm/genh.go cpu/instruction.go cpu/syscall.go
	./vm genh > cvm/cvm.h

grol_cvm: Makefile cvm/cvm.c cvm/cvm.h
	$(CC) -O3 -Wall -Wextra -pedantic -Werror -o grol_cvm cvm/cvm.c

cvm-loop: grol_cvm
	time ./grol_cvm programs/loop.vm

fact: vm grol_cvm
	./vm compile programs/fact.asm programs/itoa.asm
	./vm run -quiet programs/fact.vm
	./grol_cvm programs/fact.vm

debug-cvm: Makefile cvm/cvm.c cvm/cvm.h
	$(CC) -O3 -Wall -Wextra -pedantic -Werror -DDEBUG=1 -o grol_cvm cvm/cvm.c
	./grol_cvm programs/simple.vm
	./grol_cvm programs/addr.vm
	./grol_cvm programs/incr.vm
	./grol_cvm programs/itoa.vm

nativecloop: Makefile cvm/loop.c
	$(CC) -O3 -Wall -Wextra -pedantic -Werror cvm/loop.c
	time ./a.out programs/loop.vm
	$(CC) -O3 -Wall -Wextra -pedantic -Werror -DNOVOLATILE cvm/loop.c
	time ./a.out programs/loop.vm

elf64: vm
	$(MAKE) -C native test-loop-native

TINY_OPTS:=-opt 2
tiny_vm: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 tinygo build -o tiny_vm $(TINY_OPTS) .
	time ./tiny_vm run programs/loop.vm

vm-debug: Makefile *.go */*.go $(GEN)
	CGO_ENABLED=0 go build -tags debug -o vm-debug .

run-debug: vm-debug
	./vm-debug run -loglevel debug programs/itoa.vm

install:
	CGO_ENABLED=0 go install -trimpath -ldflags="-s" -tags "$(GO_BUILD_TAGS)" grol.io/vm@$(GIT_TAG)
	ls -lh "$(shell go env GOPATH)/bin/vm"
	vm version

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

.PHONY: all lint generate test clean run build install unit-tests elf64 timing-loop cvm-loop timing-tests
.PHONY: show_cpu_profile show_mem_profile nativecloop debug-cvm fact cat-test wc-test echo-test

show_cpu_profile:
	-pkill pprof
	go tool pprof -http :8080 ./vm cpu.pprof

show_mem_profile:
	-pkill pprof
	go tool pprof -http :8081 ./vm mem.pprof
