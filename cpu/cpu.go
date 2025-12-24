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

type Data int64

type Operation struct {
	Data Data
}

func (op *Operation) Opcode() Instruction {
	return Instruction(op.Data & 0xFF) //nolint:gosec // duh... 0xFF means it can't overflow
}

func (op *Operation) Operand() Data {
	return op.Data >> 8
}

func Op(opcode Instruction, operand Data) Operation {
	return Operation{Data: (operand << 8) | Data(opcode)}
}

func (op *Operation) SetOpcode(opcode Instruction) {
	op.Data = (op.Data &^ 0xFF) | Data(opcode)
}

func (op *Operation) SetOperand(operand Data) {
	if operand > (1<<55-1) || operand < -(1<<55) {
		panic(fmt.Sprintf("operand out of range: %d", operand))
	}
	op.Data = (op.Data & 0xFF) | (operand << 8)
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
	JNZ
	lastInstruction
)

const (
	// HEADER for the VM binary format, starts with non printable version byte to indicate it's binary.
	// The first byte is the version byte, followed by the ASCII characters "GROL VM".
	HEADER = "\x01GROL VM"
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

func execute(pc Data, program []Operation, accumulator Data) (Data, Data) {
	end := Data(len(program))
	for pc < end {
		op := program[pc]
		switch op.Opcode() {
		case Exit:
			log.Infof("Exit at PC: %d. Halting execution.", pc)
			log.Infof("Accumulator: %d, PC: %d", accumulator, pc)
			return accumulator, op.Operand()
		case Load:
			accumulator = op.Operand()
			if Debug {
				log.Debugf("Load at PC: %d, value: %d", pc, accumulator)
			}
		case Add:
			accumulator += op.Operand()
			if Debug {
				log.Debugf("Add  at PC: %d, value: %d -> %d", pc, op.Operand(), accumulator)
			}
		case JNZ:
			if accumulator != 0 {
				if Debug {
					log.Debugf("JNE   at PC: %d, jumping to PC: %d", pc, op.Operand())
				}
				pc = op.Operand()
				continue
			}
			if Debug {
				log.Debugf("JNE   at PC: %d, not jumping", pc)
			}
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
