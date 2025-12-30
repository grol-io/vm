[![GoDoc](https://godoc.org/grol.io/vm?status.svg)](https://pkg.go.dev/grol.io/vm)
[![Go Report Card](https://goreportcard.com/badge/grol.io/vm)](https://goreportcard.com/report/grol.io/vm)
[![CI Checks](https://github.com/grol-io/vm/actions/workflows/gochecks.yml/badge.svg)](https://github.com/grol-io/vm/actions/workflows/gochecks.yml)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/grol-io/vm)

# vm

Virtual Machine experiment

This is an early experiment and comparison and optimization of a miniature assembler and VM with the following minimalistic instructions:

Immediate operand instructions:
- `LoadI`, `AddI`, `SubI`, `MulI`, `DivI`, `ModI`, `ShiftI`, `AndI` (though they can also load the relative address of a label as value)

Relative address based instructions:
- `LoadR`, `AddR`, `SubR`, `MulR`, `DivR`, `StoreR`, `JNZ` (jump if not equal to 0), `JNEG` (jump if negative), `JPOS` (jump if positive or 0), `JumpR` (unconditional jump), `IncrR i addr` increments (or decrements if `i` is negative the value at `addr` by `i` and loads the result in the accumulator)

Stack-oriented instructions let the VM manage simple call frames:
- `Call` pushes the return address, and `Return` unwinds the stack (optionally dropping extra entries).
- `Push`/`Pop` move the accumulator to and from the stack while reserving or discarding extra slots.
- `LoadS`, `StoreS`, `AddS`, and `IncrS` read and write relative to the current stack pointer so stack-resident variables can be manipulated without touching memory directly, and `SysS` mirrors `Sys` but uses a stack index operand for its first argument.

Short Data/string format:
- String quoting use the go rules (ie in "double-quotes" with \ sequences or single 'x' for 1 character or backtick for verbatim)
- str8: 1 byte size, remaining data (so string 7 bytes or less are 1 word, longer is chunked into 8 bytes words)
- str16: 2 byte size (upcoming)

Syscall:
- `Sys` 8bit callid (lowest byte), 48 remaining bits as (first) argument to the syscall
  - 1: Exit with value from arg
  - 2: Sleep argument in milliseconds
  - 3: Write writes a str8 to stdout
  - more to come

It compares go, tinygo, C based VMs (and plain C loop for reference).

See [Makefile](Makefile) / run `make`

Binary release of the go version also available in releases/ or via
```sh
go install grol.io/vm@latest
```

(and docker as well)
