package cpu

import "strings"

type Instruction uint8

const (
	invalidInstruction Instruction = iota
	LoadI
	AddI
	JNZ
	LoadR
	AddR
	StoreR
	Sys
	lastInstruction
)

//go:generate stringer -type=Instruction
var _ = lastInstruction.String() // force compile error if go generate is missing.

var str2instr map[string]Instruction

func init() {
	str2instr = make(map[string]Instruction, lastInstruction)
	for i := range lastInstruction {
		str2instr[strings.ToLower(i.String())] = i
	}
}

// InstructionFromString converts a string (which must be lowercase) to an Instruction.
func InstructionFromString(s string) (Instruction, bool) {
	instr, ok := str2instr[s]
	return instr, ok
}
