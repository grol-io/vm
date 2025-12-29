package cpu

import "strings"

type Instruction uint8

const (
	InvalidInstruction Instruction = iota
	LoadI
	AddI
	SubI
	MulI
	DivI
	ModI
	ShiftI
	AndI
	JNZ  // Jump if A != 0
	JNEG // Jump if A < 0
	JPOS // Jump if A >= 0
	JumpR
	LoadR
	AddR
	SubR
	MulR
	DivR
	// no ModR, ShiftR, AndR on purpose.

	StoreR
	IncrR
	Sys
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
