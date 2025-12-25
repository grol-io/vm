[![GoDoc](https://godoc.org/grol.io/vm?status.svg)](https://pkg.go.dev/grol.io/vm)
[![Go Report Card](https://goreportcard.com/badge/grol.io/vm)](https://goreportcard.com/report/grol.io/vm)
[![CI Checks](https://github.com/grol-io/vm/actions/workflows/gochecks.yml/badge.svg)](https://github.com/grol-io/vm/actions/workflows/gochecks.yml)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/grol-io/vm)

# vm

Virtual Machine experiment

This is an early experiment and comparison and optimization of a miniature assembler and VM with the following minimalistic instructions:

Immediate operand instructions:
- `LoadI`, `AddI`, and `ExitI` (to be replaced by syscall eventually, see #14)

Relative address based instructions:
- `LoadR`, `AddR`, `StoreR`, `JNZ` (jump if not equal to 0)

It compares go, tinygo, C based VMs (and plain C loop for reference).

See [Makefile](Makefile) / run `make`

Binary release of the go version also available in releases/ or via
```sh
go install grol.io/vm@latest
```

(and docker as well)
