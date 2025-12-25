# vm

Virtual Machine experiment

This is an early experiment and comparison and optimization of a miniature assembler and VM with 4 instructions:

Immediate operand instructions:
- `LoadI`, `AddI`, and `ExitI` (to be replaced by syscall eventually, see #14)
Relative address based instructions:
- `Load`, `Add`, `Store`, `JNZ` (jump if not equal to 0)

It compares go, tinygo, C based VMs (and plain C loop for reference).

See [Makefile](Makefile) / run `make`

Binary release of the go version also available in releases/ or via
```sh
go install grol.io/vm@latest
```

(and docker as well)
