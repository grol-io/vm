package cpu

import "strings"

type Instruction uint8

const (
	InvalidInstruction Instruction = iota

	LoadI  // Load Immediate (param->A)
	AddI   // A = A + param
	SubI   // A = A - param
	MulI   // A = A * param
	DivI   // A = A / param
	ModI   // A = A % param
	ShiftI // A = A << param
	AndI   // A = A & param
	JNZ    // Jump if A != 0
	JNEG   // Jump if A < 0
	JPOS   // Jump if A >= 0
	JumpR  // Unconditional jump to relative address
	LoadR  // Load from relative address (A = *[PC + param])
	AddR   // A = A + *[PC + param]
	SubR   // A = A - *[PC + param]
	MulR   // A = A * *[PC + param]
	DivR   // A = A / *[PC + param]
	// no ModR, ShiftR, AndR on purpose.

	StoreR // *[PC + param] = A
	IncrR  // A = *[PC + param1] + param0; *[PC + param1] = A

	Call   // push PC+1 on stack and jump to PC + param
	Return // pop PC from stack and unwind stack by param additional entries (RET 0 if nothing was pushed)
	Push   // push A and reserve param additional entries on stack
	Pop    // pop A from stack + param additional entries
	LoadS  // load from stack (A = *[SP - param])
	StoreS // store to stack (*[SP - param] = A)
	AddS   // A = A + *[SP - param]
	IncrS  // A = *[SP - param1] + param0; *[SP - param1] = A

	LoadAA // load absolute address
	LoadB  // byte offset from addr
	StoreB // store byte at addr

	Sys  // syscall with immediate or relative address operand
	SysS // syscall with stack index operand
	LastInstruction
)

//go:generate stringer -type=Instruction
var _ = LastInstruction.String() // force compile error if go generate is missing.

var str2instr map[string]Instruction

func init() {
	str2instr = make(map[string]Instruction, LastInstruction)
	for i := InvalidInstruction + 1; i < LastInstruction; i++ {
		str2instr[strings.ToLower(i.String())] = i
	}
}

// InstructionFromString converts a string (which must be lowercase) to an Instruction.
func InstructionFromString(s string) (Instruction, bool) {
	instr, ok := str2instr[s]
	return instr, ok
}
