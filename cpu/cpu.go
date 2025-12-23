package cpu

import "fortio.org/log"

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
	Add
)

func Run(files ...string) {
	log.Infof("Running files: %v", files)
	// TODO: Implement the CPU execution logic
}
