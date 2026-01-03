[![GoDoc](https://godoc.org/grol.io/vm?status.svg)](https://pkg.go.dev/grol.io/vm)
[![Go Report Card](https://goreportcard.com/badge/grol.io/vm)](https://goreportcard.com/report/grol.io/vm)
[![CI Checks](https://github.com/grol-io/vm/actions/workflows/gochecks.yml/badge.svg)](https://github.com/grol-io/vm/actions/workflows/gochecks.yml)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/grol-io/vm)

# vm

Virtual Machine experiment

This is an early experiment and comparison and optimization of a miniature assembler and VM with the following (sort of but less and less minimalistic) instructions:

Immediate operand instructions:
- `LoadI`, `AddI`, `SubI`, `MulI`, `DivI`, `ModI`, `ShiftI`, `AndI` (though they can also load the relative address of a label as value)

Relative address based instructions:
- `LoadR`, `AddR`, `SubR`, `MulR`, `DivR`, `StoreR`, `JNZ` (jump if not equal to 0), `JNEG` (jump if negative), `JPOS` (jump if positive or 0), `JumpR` (unconditional jump), `IncrR i addr` increments (or decrements if `i` is negative the value at `addr` by `i` and loads the result in the accumulator)

Stack-oriented instructions let the VM manage simple call frames:
- `Call` pushes the return address, and `Ret` unwinds the stack (optionally dropping extra entries).
- `Push`/`Pop` move the accumulator to and from the stack while reserving or discarding extra slots.
- `LoadS`, `StoreS`, `AddS`, `SubS`, `MulS`, `DivS`, and `IncrS` read and write relative to the current stack pointer so stack-resident variables can be manipulated without touching memory directly, and `SysS` mirrors `Sys` but uses a stack index operand for its first argument.
- `IdivS` divides the stack location by the accumulator and keeps the remainder in A.
- `StoreSB` stores a single byte from the accumulator into a stack-resident buffer: the first operand specifies the base stack offset of the target word span, while 2nd operand indicates a stack slot containing the byte offset (which can be more than 8). The handler computes the word/bit position and patches the selected byte in place. It is handy for building packed `str8` buffers on the stack (see [programs/itoa.asm](programs/itoa.asm)).

Short Data/string format:
- String quoting use the go rules (ie in "double-quotes" with \ sequences or single 'x' for 1 character or backtick for verbatim)
- str8: 1 byte size, remaining data (so string 7 bytes or less are 1 word, longer is chunked into 8 bytes words)
- str16: 2 byte size (upcoming)

Syscall:
- `Sys` 8bit callid (lowest byte), 48 remaining bits as (first) argument to the syscall
  - `Exit` (1) with value from arg
  - `Read` (2) reads from stdin up to A bytes into param address/stack buffer.
  - `Write` (3) writes a str8 to stdout - in the SysS variant the accumulator is a byte offset from the passed stack offset.
  - `Sleep` (4) argument in milliseconds

Assembler only:
- `data` for a 64 bit word
- `str8` for string (with the double or backtick quotes)
- on a line preceding an instruction: _label_ + `:` label for the *R instruction (relative address calculation). _label_ starts with a letter.
- `Var v1 v2 ...` virtual instruction that generates a `Push` instruction with the number of identifiers provided and defines labels for said variables starting at 0 (which will start with the value of the accumulator while the rest will start 0 initialized).
- `Param p1 p2 ...` virtual instruction that generates stack labels for p1, p2 as offset from before the return PC (ie parameters pushed (via `Var` or `Push`) by the caller before calling `Call`)
- `Return` virtual instruction that generates a `Ret n` where _n_ is such as a Var push is undone.

## Benchmarks
Compares go, tinygo, C based VMs (and plain C loop for reference).

## Usage and more

See [Makefile](Makefile) / run `make`

## Installation

Binary release of the go version also available in releases/ or via
```sh
go install grol.io/vm@latest
```

(and homebrew and docker as well)
