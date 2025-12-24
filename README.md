# vm

Virtual Machine experiment

This is an early experiment and comparison and optimization of a miniature assembler and VM with 4 instructions:

load, add, jnz (jump if not equal to 0), exit

It compares go, tinygo, C based VMs (and plain C loop for reference).

See [Makefile](Makefile) / run `make`

Binary release of the go version also available in releases/ or via
```sh
go install grol.io/vm@latest
```

(and docker as well)
