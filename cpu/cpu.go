package cpu

import (
	"encoding/binary"
	"errors"
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
	c.Program = p[0:len(p):len(p)] // force panic if program is too short/invalid.
	c.PC = 0
	// We keep the accumulator value intact when loading a new program
	if len(p) == 0 {
		return errors.New("program is empty")
	}
	return nil
}

func readInt64(b []byte) int64 {
	return int64(binary.LittleEndian.Uint64(b)) //nolint:gosec // binary cast
}

// ReadInt64 reads the next 8 bytes from the program as an int64 value.
func (c *CPU) ReadInt64() (v int64) {
	// It's ok to panic if the program does not have enough bytes remaining.
	v = readInt64(c.Program[c.PC : c.PC+8])
	c.PC += 8
	return v
}

func (c *CPU) Execute() error {
	// TODO: Implement the CPU execution logic
	for c.PC < uint64(len(c.Program)) {
		instr := Instruction(c.Program[c.PC])
		c.PC++
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
	}
	log.Warnf("Program terminated without explicit Abort instruction. Accumulator: %d, PC: %d", c.Accumulator, c.PC)
	return nil
}
