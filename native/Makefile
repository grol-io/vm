hello-asm: ../vm
	../vm compile -elf64 ./sample/hello.asm
	mv ./sample/hello ./prg

../vm:
	$(MAKE) -C .. vm
hello-go:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s" -trimpath -o prg ./sample

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

.PHONY: hello-asm hello-go docker-build container-build
