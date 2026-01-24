# Makefile for native ELF64 binary generation and testing
#
# Requires Docker for testing (Linux x86-64 emulation on macOS/arm64)
# Use: make test          - run all tests
#      make test-hello    - test hello.asm only
#      make docker-build  - build and test Docker container

SHELL := /bin/bash
VM := ../vm

# Platform detection
OS := $(shell uname -s)
ARCH := $(shell uname -m)

# Docker settings
DOCKER_IMAGE := alpine:latest
DOCKER_PLATFORM := linux/amd64

# All sample programs
ASM_FILES := $(wildcard sample/*.asm)
ELF_FILES := $(ASM_FILES:.asm=)

.PHONY: all clean test test-hello test-counter test-loop test-loop-native hello-asm hello-go docker-build container-build

all: test-all $(ELF_FILES)

test-all: test-hello test-counter test-loop test-loop-native

# Pattern rule: .asm -> native binary (no extension)
sample/%: sample/%.asm $(VM)
	$(VM) compile -elf64 $<

$(VM):
	$(MAKE) -C .. vm

# Compile hello.asm and copy to prg for Docker build
hello-asm: sample/hello
	cp sample/hello ./prg

hello-go:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s" -trimpath -o prg ./sample

# === Tests ===
# Run all tests in Docker
test: test-hello test-counter test-loop

define docker-test
	@echo "Testing $(1)..."
	@docker run --rm --platform $(DOCKER_PLATFORM) \
		-v "$$PWD/sample:/sample:ro" $(DOCKER_IMAGE) \
		sh -c 'time /sample/$(1); result=$$?; \
	if [ "$$result" = "$(2)" ]; then \
		echo "  PASS: exit code $$result (expected $(2))"; \
	else \
		echo "  FAIL: exit code $$result (expected $(2))"; \
		exit 1; \
	fi'
endef

test-hello: sample/hello
	$(call docker-test,hello,42)

test-counter: sample/counter
	$(call docker-test,counter,43)

# Loop test: 1 billion iterations (~0.3s under Rosetta emulation)
test-loop: sample/loop
	$(call docker-test,loop,0)

# Native test (only works on Linux x86_64)
test-loop-native: sample/loop_aligned
ifeq ($(OS)/$(ARCH),Linux/x86_64)
	@echo "Running loop natively on $(OS)/$(ARCH)..."
	time ./sample/loop_aligned; echo "  Exit code: $$?"
else
	@echo "Native test not supported on $(OS)/$(ARCH) (requires Linux/x86_64)"
endif

# === Docker Container Build ===
BASE_TAG:=ghcr.io/grol-io/vm-base
VERSION:=latest
TAG:=$(BASE_TAG):$(VERSION)

PRG:=hello-asm # hello-go

docker-build: $(PRG)
	docker buildx build --platform linux/amd64 --load -t $(TAG) .
	docker image ls $(TAG)
	docker image inspect $(TAG) --format '{{.Os}}/{{.Architecture}}'
	docker run --platform linux/amd64 --rm $(TAG); test $$? -eq 42

container-build: $(PRG)
	container build --arch x86_64 -t $(TAG) .
	container image list -v|grep $(BASE_TAG)
	container image inspect $(TAG)| \
		jq -r '.[].variants[] | "\(.platform.os)/\(.platform.architecture)"'
	container run --arch x86_64 --rm $(TAG);code=$$?; echo "Exit code is $$code"; test $$code -eq 42

clean:
	rm -f sample/hello sample/counter sample/loop prg
