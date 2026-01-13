# Instruction Encoding Examples

## Standard Instruction (56 remaining bits)
```
Example: LoadI 42

┌────────┬─────────────────────────────────────────────────────────┐
│ Opcode │                  Operand (56 bits)                      │
│ 8 bits │                  Signed value: 42                       │
└────────┴─────────────────────────────────────────────────────────┘
  LoadI                         42

remainingBits = 56 (default)
```

## Single Packed Argument (48 remaining bits)
```
Example: SysS exit 0

┌────────┬───────────┬──────────────────────────────────────────────┐
│ Opcode │  Syscall  │        Operand (48 bits)                     │
│ 8 bits │   8 bits  │        Signed value: 0                       │
└────────┴───────────┴──────────────────────────────────────────────┘
  SysS      exit=1              0

remainingBits = 48 (after packing syscall ID)
```

```
Example: JNE 0 loop

┌────────┬───────────┬──────────────────────────────────────────────┐
│ Opcode │   Value   │        Jump Offset (48 bits)                 │
│ 8 bits │   8 bits  │        Relative PC to 'loop'                 │
└────────┴───────────┴──────────────────────────────────────────────┘
   JNE    compare=0           -25 (example)

remainingBits = 48 (after packing compare value)
```

```
Example: IncrR 1 counter

┌────────┬───────────┬──────────────────────────────────────────────┐
│ Opcode │   Delta   │        Address (48 bits)                     │
│ 8 bits │   8 bits  │        Relative PC to 'counter'              │
└────────┴───────────┴──────────────────────────────────────────────┘
  IncrR    delta=1            +10 (example)

remainingBits = 48 (after packing increment delta)
```

## Triple Packed Argument (32 remaining bits) - Upcoming
```
Sys Read Fd Len DstAddr

┌────────┬───────────┬───────────┬───────────┬────────────────────┐
│ Opcode │ SyscallId │   FD      │   Len     │   Addr (32 bits)   │
│ 8 bits │   8 bits  │   8 bits  │   8 bits  │   Operand value    │
└────────┴───────────┴───────────┴───────────┴────────────────────┘
 Sys        Read       Fd=0 (stdin)  Len=128   Buf=buffer address

remainingBits = 32 (after packing three 8-bit args)
```

## Implementation Flow

### Assembler (asm/asm.go)
```go
// 1. Initialize
remainingBits := 56  // default: full operand

// 2. Pack arguments (example: IncrR)
v := parseValue(args[0])       // Parse first argument
op = op.SetOperand(ImmediateData(v))   // Pack it (takes 8 bits)
remainingBits = 48             // 48 bits remain

label = args[1]                // Second arg is label (resolved later)

// 3. Store in Line struct
result = append(result, Line{
    Op:            op,
    Label:         label,
    Data:          false,
    remainingBits: 48,         // Document how many bits remain
})

// 4. Resolve labels in emitCode()
relativePC := targetPC - pc
// Use SetOperandWithBits with the tracked remainingBits value
op = op.SetOperandWithBits(relativePC, line.remainingBits)
```

### CPU Execution (cpu/cpu.go)
```go
// Extract packed arguments from operand
operand := op.OperandInt64()

// For single 8-bit packed:
arg1 := operand & 0xFF          // Extract lower 8 bits
remaining := operand >> 8       // Remaining 48 bits

// For double 8-bit packed:
arg1 := operand & 0xFF          // Extract bits 0-7
arg2 := (operand >> 8) & 0xFF   // Extract bits 8-15
remaining := operand >> 16      // Remaining 40 bits

// For triple 8-bit packed:
arg1 := operand & 0xFF          // Extract bits 0-7
arg2 := (operand >> 8) & 0xFF   // Extract bits 8-15
arg3 := (operand >> 16) & 0xFF  // Extract bits 16-23
remaining := operand >> 24      // Remaining 32 bits

// For signed 8-bit values:
signedArg := int64(int8(operand & 0xFF))
```

## Bit Width Decision Guide

Choose based on address range needs:
- **56 bits**: ±36 trillion range (standard instructions)
- **48 bits**: ±140 trillion range (1 packed arg) - still enormous
- **40 bits**: ±549 billion range (2 packed args) - plenty for PC offsets
- **32 bits**: ±2 billion range (3 packed args) - good for most programs
- **24 bits**: ±8 million range (4 packed args) - acceptable for small programs
- **16 bits**: ±32K range (5 packed args) - local jumps only

Current VM programs are < 1000 instructions, so even 32 bits is overkill!
