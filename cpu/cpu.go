package cpu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"fortio.org/log"
)

type ImmediateData int64 // Signed extended 56 bits really.

type Operation int64

func (op Operation) Opcode() Instruction {
	return Instruction(op & 0xFF) //nolint:gosec // duh... 0xFF means it can't overflow
}

func (op Operation) Operand() ImmediateData {
	return ImmediateData(op >> 8)
}

func (op Operation) OperandInt64() int64 {
	return int64(op >> 8)
}

func (op Operation) SetOpcode(opcode Instruction) Operation {
	return (op &^ 0xFF) | Operation(opcode)
}

func (op Operation) SetOperand(operand ImmediateData) Operation {
	if operand > ((1<<55)-1) || operand < -(1<<55) {
		panic(fmt.Sprintf("operand out of range: %d", operand))
	}
	return (op & 0xFF) | (Operation(operand) << 8)
}

type CPU struct {
	Accumulator int64
	PC          ImmediateData
	// SP          uint64
	Program []Operation
}

type Instruction uint8

const (
	Exit Instruction = iota
	LoadI
	AddI
	JNZ
	Load
	Add
	Store
	lastInstruction
)

const (
	// HEADER for the VM binary format, starts with non printable version byte to indicate it's binary.
	// The first byte is the version byte, followed by the ASCII characters "GROL VM".
	HEADER = "\x01GROL VM"
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

// InstructionFromString converts a lower case string to an Instruction.
func InstructionFromString(s string) (Instruction, bool) {
	instr, ok := str2instr[s]
	return instr, ok
}

func Run(files ...string) int {
	cpu := &CPU{}
	log.Infof("Starting CPU - size of operation: %d bytes", binary.Size(Operation(0)))
	for _, file := range files {
		log.Infof("Running file: %s", file)
		f, err := os.Open(file)
		if err != nil {
			return log.FErrf("Failed to read file %s: %v", file, err)
		}
		defer f.Close()
		header := make([]byte, len(HEADER))
		_, err = f.Read(header)
		if err != nil {
			return log.FErrf("Failed to read header from file %s: %v", file, err)
		}
		if string(header) != HEADER {
			return log.FErrf("Invalid header in file %s: %q", file, string(header))
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

func execute(pc ImmediateData, program []Operation, accumulator int64) (int64, int64) {
	end := ImmediateData(len(program))
	for pc < end {
		op := program[pc]
		switch op.Opcode() {
		case Exit:
			log.Infof("Exit at PC: %d. Halting execution.", pc)
			log.Infof("Accumulator: %d (%x), PC: %d", accumulator, accumulator, pc)
			return accumulator, op.OperandInt64()
		case LoadI:
			accumulator = op.OperandInt64()
			if Debug {
				log.Debugf("LoadI   at PC: %d, value: %d", pc, accumulator)
			}
		case AddI:
			accumulator += op.OperandInt64()
			if Debug {
				log.Debugf("AddI   at PC: %d, value: %d -> %d", pc, op.OperandInt64(), accumulator)
			}
		case JNZ:
			if accumulator != 0 {
				if Debug {
					log.Debugf("JNZ    at PC: %d, jumping to PC: %d", pc, op.OperandInt64())
				}
				pc += op.Operand()
				continue
			}
			if Debug {
				log.Debugf("JNZ    at PC: %d, not jumping", pc)
			}
		case Load:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			accumulator = int64(program[pc+offset])
			if Debug {
				log.Debugf("Load   at PC: %d, offset: %d, value: %d", pc, offset, accumulator)
			}
		case Add:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			value := int64(program[pc+offset])
			accumulator += value
			if Debug {
				log.Debugf("Add    at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		case Store:
			offset := op.Operand()
			if Debug {
				oldValue := int64(program[pc+offset]) // may panic if offset is out of bounds, that's fine
				log.Debugf("Store  at PC: %d, offset: %d, old value: %d, new value: %d", pc, offset, oldValue, accumulator)
			}
			// ok to panic if offset is out of bounds
			program[pc+offset] = Operation(accumulator)
		default:
			log.Errf("unknown instruction: %v at PC: %d (%x)", op.Opcode(), pc, op)
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
