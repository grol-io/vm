package cpu

import "strings"

type Instruction uint8

const (
	invalidInstruction Instruction = iota
	LoadI
	AddI
	SubI
	MulI
	DivI
	ModI
	ShiftI
	AndI
	JNZ
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
	lastInstruction
)

//go:generate stringer -type=Instruction
var _ = lastInstruction.String() // force compile error if go generate is missing.

var str2instr map[string]Instruction

func init() {
	str2instr = make(map[string]Instruction, lastInstruction)
	for i := invalidInstruction + 1; i < lastInstruction; i++ {
		str2instr[strings.ToLower(i.String())] = i
	}
}

// InstructionFromString converts a string (which must be lowercase) to an Instruction.
func InstructionFromString(s string) (Instruction, bool) {
	instr, ok := str2instr[s]
	return instr, ok
}
