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

func execute(pc Data, program []Operation, accumulator Data) (Data, Data) {
	end := Data(len(program))
	for pc < end {
		op := program[pc]
		switch op.Opcode {
		case Exit:
			log.Infof("Exit at PC: %d. Halting execution.", pc)
			log.Infof("Accumulator: %d, PC: %d", accumulator, pc)
			return accumulator, op.Data
		case Load:
			accumulator = op.Data
		case Add:
			accumulator += op.Data
		case JNE:
			if accumulator != 0 {
				pc = op.Data
				continue
			}
		default:
			log.Errf("unknown instruction: %v at PC: %d", op.Opcode, pc)
			return accumulator, -1
		}
		pc++
	}
	log.Warnf("Program terminated without explicit Exit instruction. Accumulator: %d, PC: %d", accumulator, pc)
	return accumulator, 0
}

func (c *CPU) Execute() int {
	accumulator, exitCode := execute(c.PC, c.Program, c.Accumulator)
	c.Accumulator = accumulator
	return int(exitCode)
}
