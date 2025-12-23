package cpu

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"

	"fortio.org/log"
)

const NumRegs = 16

type CPU struct {
	Accumulator int64
	PC          uint64
	// SP          uint64
	Program []byte
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
	cpu := &CPU{}
	for _, file := range files {
		log.Infof("Running file: %s", file)
		p, err := os.ReadFile(file)
		if err != nil {
			return log.FErrf("Failed to read file %s: %v", file, err)
		}
		err = cpu.LoadProgram(p)
		if err != nil {
			return log.FErrf("Failed to load program %s: %v", file, err)
		}
		err = cpu.Execute()
		if err != nil {
			return log.FErrf("Failed to execute program %s: %v", file, err)
		}
	}
	return 0
}

func (c *CPU) LoadProgram(p []byte) error {
	c.Program = p
	c.PC = 0
	// for now (see issue #8)
	if len(p)%9 != 0 {
		return fmt.Errorf("program length must be a multiple of 9 bytes")
	}
	return nil
}

func ReadInt64(b []byte) int64 {
	return int64(binary.LittleEndian.Uint64(b)) //nolint:gosec // binary cast
}

// ReadInt64 reads the next 8 bytes from the program as an int64 value.
// it's ok to panic if the program does not have enough bytes remaining.
func (c *CPU) ReadInt64() int64 {
	return ReadInt64(c.Program[c.PC+1 : c.PC+9])
}

func (c *CPU) Execute() error {
	// TODO: Implement the CPU execution logic
	for c.PC < uint64(len(c.Program)) {
		instr := Instruction(c.Program[c.PC])
		switch instr {
		case Abort:
			log.Infof("Abort instruction encountered. Halting execution.")
			log.Infof("Accumulator: %d, PC: %d", c.Accumulator, c.PC)
			return nil
		case Load:
			readValue := c.ReadInt64() // Read the next 8 bytes as the value
			c.Accumulator = readValue
			log.Debugf("Load instruction encountered at PC: %d, value: %d", c.PC, c.Accumulator)
		case Add:
			readValue := c.ReadInt64() // Read the next 8 bytes as the value
			c.Accumulator += readValue
			log.Debugf("Add instruction encountered at PC: %d, value: %d -> %d", c.PC, readValue, c.Accumulator)
		default:
			return fmt.Errorf("unknown instruction: %v", instr)
		}
		c.PC += 9 // 1 code + 8 bytes of data/operand
	}
	return nil
}
