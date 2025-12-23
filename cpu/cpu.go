package cpu

import (
	"strings"

	"fortio.org/log"
)

const NumRegs = 16

type CPU struct {
	IntRegs  [NumRegs]int64
	AddrRegs [NumRegs]uint64
	PC       uint64
	SP       uint64
}

type Instruction uint8

const (
	Abort Instruction = iota
	Load
	Add
	lastInstruction
)

//go:generate stringer -type=Instruction
var _ = Add.String() // force compile error if go generate is missing.

var str2instr map[string]Instruction

func init() {
	str2instr = make(map[string]Instruction, lastInstruction)
	for i := range lastInstruction {
		str2instr[strings.ToLower(i.String())] = i
	}
}

func InstructionFromString(s string) (Instruction, bool) {
	instr, ok := str2instr[strings.ToLower(s)]
	return instr, ok
}

func Run(files ...string) int {
	log.Infof("Running files: %v", files)
	// TODO: Implement the CPU execution logic
	return 0
}
