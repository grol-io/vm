package cpu

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"

	"fortio.org/log"
)

type Data int64

type Operation struct {
	Data   Data
	Opcode Instruction
	// padding makes it even slower (!) despite 16bytes alignment
	// Ex1, Ex2, Ex3, Ex4, Ex5, Ex6, Ex7 Instruction
}

type CPU struct {
	Accumulator Data
	PC          Data
	// SP          uint64
	Program []Operation
}

type Instruction uint8

const (
	Exit Instruction = iota
	Load
	Add
	JNE
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
	log.Infof("Starting CPU - size of operation: %d bytes", binary.Size(Operation{}))
	for _, file := range files {
		log.Infof("Running file: %s", file)
		f, err := os.Open(file)
		if err != nil {
			return log.FErrf("Failed to read file %s: %v", file, err)
		}
		err = cpu.LoadProgram(f)
		if err != nil {
			return log.FErrf("Failed to load program %s: %v", file, err)
		}
		execResult := cpu.Execute()
		if execResult != 0 {
			log.Warnf("No 0 exit of program %s: %v", file, execResult)
			return execResult
		}
	}
	return 0
}

func (c *CPU) LoadProgram(f *os.File) error {
	var op Operation
	for {
		err := binary.Read(f, binary.LittleEndian, &op)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		c.Program = append(c.Program, op)
	}
	return nil
}

func (c *CPU) Execute() int {
	end := Data(len(c.Program))
	for c.PC < end {
		pc := c.PC
		op := c.Program[pc]
		instr := op.Opcode
		data := op.Data
		c.PC++
		switch instr {
		case Exit:
			log.Infof("Exit at PC: %d. Halting execution.", pc)
			log.Infof("Accumulator: %d, PC: %d", c.Accumulator, c.PC)
			return int(data)
		case Load:
			c.Accumulator = data
			if Debug {
				log.Debugf("Load at PC: %d, value: %d", pc, c.Accumulator)
			}
		case Add:
			c.Accumulator += data
			if Debug {
				log.Debugf("Add  at PC: %d, value: %d -> %d", pc, data, c.Accumulator)
			}
		case JNE:
			targetPC := data
			if c.Accumulator != 0 {
				if Debug {
					log.Debugf("JNE   at PC: %d, jumping to PC: %d", pc, targetPC)
				}
				c.PC = targetPC
			} else if Debug {
				log.Debugf("JNE   at PC: %d, not jumping", pc)
			}
		default:
			log.Errf("unknown instruction: %v", instr)
			return -1
		}
	}
	log.Warnf("Program terminated without explicit Exit instruction. Accumulator: %d, PC: %d", c.Accumulator, c.PC)
	return 0
}
