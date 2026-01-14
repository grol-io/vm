# Instruction Argument Packing

## Overview

Instructions in the Grol VM are 64-bit words consisting of an 8-bit opcode and a 56-bit operand field. To support multiple arguments, we pack smaller values into the operand field.

## Current Implementation

The `Line.remainingBits` field tracks how many bits remain available for the final operand/address field after packing earlier arguments.

### Bit Layout

- **Default (no packing)**: 8-bit opcode + 56-bit operand
  - `remainingBits = 56`

- **One 8-bit argument packed**: 8-bit opcode + 8-bit arg1 + 48-bit operand
  - `remainingBits = 48`
  - Examples: `Sys`, `SysS`, conditional jumps (`JNE`, `JEQ`, etc.), `IncrR`, `IncrS`, `LoadSB`, `StoreSB`

- **Two 8-bit arguments packed**: 8-bit opcode + 8-bit arg1 + 8-bit arg2 + 40-bit operand
  - `remainingBits = 40`
  - Currently no instructions use this, but the infrastructure supports it

- **Three 8-bit arguments packed**: 8-bit opcode + 8-bit arg1 + 8-bit arg2 + 8-bit arg3 + 32-bit operand
  - `remainingBits = 32`
  - Future expansion

## Examples

### Syscall Instructions (48-bit remaining)
```
Sys exit 42
```
Layout: `[8-bit Sys opcode][8-bit syscall ID][48-bit operand (42)]`

### Conditional Jumps (48-bit remaining)
```
JNE 0 loop
```
Layout: `[8-bit JNE opcode][8-bit compare value (0)][48-bit jump offset]`

### Increment with Immediate (48-bit remaining)
```
IncrR 1 counter
```
Layout: `[8-bit IncrR opcode][8-bit increment value (1)][48-bit address]`

### Load/Store Byte (48-bit remaining)
```
LoadSB buf idx
```
Layout: `[8-bit LoadSB opcode][8-bit byte offset stack index][48-bit base stack offset]`

## Future: Variable-Length Arguments

The `remainingBits` field enables future instructions with variable numbers of arguments. For example:

```
Sys Read fd len destination
```

This could be encoded as:
- Layout: `[8-bit opcode][8-bit arg1][8-bit arg2][8-bit arg3][32-bit destination]`
- `remainingBits = 32`

## Implementation Details

### In the Assembler (asm/asm.go)

1. When parsing an instruction, `remainingBits` defaults to 56
2. Each time an 8-bit value is packed, subtract 8 from `remainingBits`:
   - First pack: `remainingBits = 48`
   - Second pack: `remainingBits = 40`
   - Third pack: `remainingBits = 32`
3. In `emitCode()`, use `SetOperandWithBits(operand, remainingBits)` to set the final operand/address with the appropriate number of bits

### In the CPU (cpu/cpu.go)

To extract packed arguments, use bit shifting and masking:

```go
// For single 8-bit packed argument (e.g., syscalls):
arg1 := operand & 0xFF
remaining := operand >> 8

// For two 8-bit packed arguments:
arg1 := operand & 0xFF
arg2 := (operand >> 8) & 0xFF
remaining := operand >> 16

// For three 8-bit packed arguments:
arg1 := operand & 0xFF
arg2 := (operand >> 8) & 0xFF
arg3 := (operand >> 16) & 0xFF
remaining := operand >> 24
```

For signed 8-bit values, use:
```go
arg1 := int64(int8(operand & 0xFF))
```

## Adding New Multi-Argument Instructions

To add a new instruction with multiple packed arguments:

1. Define the instruction enum in `cpu/instruction.go`
2. Update `asm/asm.go`:
   - Add a case in the `compile()` switch statement
   - Parse and validate each argument
   - Pack arguments using `SetOperand()` and subsequent setters
   - Set `remainingBits` appropriately (56 - 8*num_packed_args)
3. Implement execution in `cpu/cpu.go`
4. Regenerate C header: `make cvm/cvm.h`
5. Implement in C VM (`cvm/cvm.c`)
6. Add tests and update documentation

## Constraints

- Each packed argument currently uses 8 bits (range: -128 to 127 for signed, 0-255 for unsigned)
- Maximum number of packed arguments: 7 (leaving 0 bits for final operand)
- More practically: 3-4 packed arguments (leaving 24-32 bits for addresses/offsets)
