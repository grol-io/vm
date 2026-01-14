# Copilot Instructions: Grol VM

Purpose: help AI agents be productive quickly in this miniature assembler + VM project.

## Architecture & Data Flow
- Assembler: parses `.asm` → emits `.vm` bytecode. See [asm/asm.go](../asm/asm.go).
- Go VM: reference CPU + runtime that executes `.vm`. See [cpu/cpu.go](../cpu/cpu.go).
- C VM: performance-focused executor for the same bytecode. See [cvm/cvm.c](../cvm/cvm.c).
- Shared enums: instructions/syscalls defined in Go and generated for C. See [cpu/instruction.go](../cpu/instruction.go), [cpu/syscall.go](../cpu/syscall.go), generated header [cvm/cvm.h](../cvm/cvm.h).
- Typical flow: `./vm compile programs/foo.asm` → `programs/foo.vm` → run via `./vm run ...` or `./grol_cvm ...`.

## Build & Run (Make targets encapsulate workflows)
- Build binaries: `make vm grol_cvm`
- Generate helpers: `make generate` (stringers), `make cvm/cvm.h` (C header via `./vm genh`).
- Run examples & cross-check Go/C parity: `make test` (includes `echo-test`, `wc-test`, `cat-test`, etc.).
- Useful debug/perf: `make vm-debug` (Go `-tags debug`), `make cvm-loop`, `make native`, `make show_cpu_profile`.
- Default Go build tags: `no_net,no_pprof` (see Makefile `GO_BUILD_TAGS`). Use `-tags debug` for extra logging.
- TinyGo variant: `make tiny_vm` builds a TinyGo binary and runs a loop benchmark.

## Bytecode & Execution Model (project specifics)
- File header: `"\x01GROL VM"` validated by loader (see [cpu/cpu.go](../cpu/cpu.go)).
- One 64-bit word per operation: low 8 bits opcode, upper 56 bits signed operand. Some ops pack multiple fields:
  - Syscalls (`Sys`/`SysS`): opcode (8 bits) | syscall ID (8 bits) | operand (48 bits signed)
  - Conditional jumps (`JNE`/`JEQ`/`JLT`/`JGT`/`JGTE`/`JLTE`): opcode (8 bits) | compare value (8 bits) | jump offset (48 bits signed)
  - `IncrR`/`IncrS`: opcode (8 bits) | increment delta (8 bits signed) | address/stack index (48 bits)
  - `LoadSB`/`StoreSB`: opcode (8 bits) | byte offset stack index (8 bits) | base stack offset (48 bits)
  - The `Line` struct in the assembler tracks `remainingBits` (56 for full operand, 48 after packing one 8-bit arg, 40 after two, etc.) to support variable numbers of packed arguments. See [PACKING.md](../PACKING.md) for details.
- Memory model: program buffer doubles as memory; PC-relative addressing reads embedded data/strings.
- Stack: fixed-size (see `StackSize`) with simple call frames; many ops have stack-relative variants (`*S`).

## Instruction & Syscall Patterns
- Immediate ALU: `LoadI`, `AddI`, `SubI`, `MulI`, `DivI`, `ModI`, `ShiftI`, `AndI`.
- PC-relative memory/ctrl: `LoadR`, `StoreR`, `AddR`/`SubR`/`MulR`/`DivR`, `JumpR`, conditional jumps.
- Stack ops: `Call`/`Ret`, `Push`/`Pop`, `LoadS`/`StoreS`/`AddS`/`SubS`/`MulS`/`DivS`/`IncrS`, `IdivS`.
- Byte access for packed buffers: `LoadSB`, `StoreSB` (assemble str8 on stack; see [programs/itoa.asm](../programs/itoa.asm)).
- Syscalls (`Sys`/`SysS`): `Exit`, `Read8`, `Write8`, `ReadN`, `WriteN`, `Sleep` (IDs in [cpu/syscall.go](../cpu/syscall.go)). `Write8` special case: in `Sys`, if operand==0, uses `A` as absolute program address; in `SysS`, `A` is byte offset from stack base operand.

## Assembler Conventions
- Labels: `label:`; operands for `*R` and control-flow can be label-relative.
- Virtual directives: `data <int64>`, `str8 "..."` (length-prefixed, chunked into 64-bit words), `.space N`, `.const NAME VALUE` (define a named constant; can reference earlier constants; redefinition with a different value errors). Constants can be used anywhere a numeric value is accepted (immediates, `.space`, stack indices for `*S`, jump compare values, etc.).
- Stack declarations: `Var v1 v2 ...` (emits `Push` and defines stack labels), `Param p1 p2 ...` (caller-pushed parameter labels), `Return` (emits `Ret` to unwind `Var`).
- argv initialization: host pushes argument addresses in reverse order, then `argc`; `SP` points at `argc`. See [programs/echo.asm](../programs/echo.asm).

## Cross-Language Sync (critical integration)
- Single source of truth: update enums in Go ([cpu/instruction.go](../cpu/instruction.go), [cpu/syscall.go](../cpu/syscall.go)).
- Regenerate artifacts: `make generate` (stringers) then `make cvm/cvm.h` (header via `./vm genh`).
- Implement execution in both runtimes: Go switch in [cpu/cpu.go](../cpu/cpu.go), C switch in [cvm/cvm.c](../cvm/cvm.c).
- Verify parity: add/update sample `.asm` under [programs/](../programs) and run `make test`.

## Generated Artifacts
- Stringers: [cpu/instruction_string.go](../cpu/instruction_string.go), [cpu/syscall_string.go](../cpu/syscall_string.go) via `make generate`.
- C header: [cvm/cvm.h](../cvm/cvm.h) via `make cvm/cvm.h` (runs `./vm genh`).

## Go/C Execution Notes
- Go VM: loader validates header, loads ops, then `SetArgs()` serializes argv as `str8` at end of program, pushes their addresses in reverse order, then pushes `argc`; `SP` points at `argc` (see `cpu/cpu.go`).
- C VM: reads header and ops, appends argv as `str8` to the program buffer, pushes addresses and `argc` with the same layout; uses generated enums from [cvm/cvm.h](../cvm/cvm.h) (see `cvm/cvm.c`).

## CLI Usage (quick examples)
```sh
# Build Go & C VMs
make vm grol_cvm

# Compile and run echo on both VMs
./vm compile programs/echo.asm
./vm run -quiet programs/echo.vm hello world
./grol_cvm programs/echo.vm hello world

# Generate helpers and C header
make generate
make cvm/cvm.h
```

## When Adding Instructions or Syscalls (checklist)
1) Update Go enums/source; 2) regenerate stringers; 3) update assembler encoding ([asm/asm.go](../asm/asm.go)); 4) implement Go exec ([cpu/cpu.go](../cpu/cpu.go)); 5) regenerate C header; 6) implement C exec ([cvm/cvm.c](../cvm/cvm.c)); 7) add example/tests under [programs/](../programs) and run `make test`.

For deeper semantics, read execution switches in [cpu/cpu.go](../cpu/cpu.go) and [cvm/cvm.c](../cvm/cvm.c), and encoding in [asm/asm.go](../asm/asm.go).
