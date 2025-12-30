package cpu

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

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

func (op Operation) Set48BitsOperand(operand ImmediateData) Operation {
	if operand > ((1<<47)-1) || operand < -(1<<47) {
		panic(fmt.Sprintf("48-bit operand out of range: %d", operand))
	}
	return (op & 0xFFFF) | (Operation(operand) << 16)
}

type CPU struct {
	Accumulator int64
	PC          ImmediateData
	// SP          uint64
	Program []Operation
}

const (
	// HEADER for the VM binary format, starts with non printable version byte to indicate it's binary.
	// The first byte is the version byte, followed by the ASCII characters "GROL VM".
	HEADER = "\x01GROL VM"
)

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
			log.Warnf("Non 0 exit of program %s: %v", file, execResult)
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

const unknownSyscallAbortCode = 99

// sysPrint prints the str8 bytes and returns the number of bytes it did output.
func sysPrint(out io.Writer, memory []Operation, addr int) int64 {
	op := memory[addr]
	l := int64(op & 0xFF)
	if l == 0 {
		return 0
	}
	buf := bytes.Buffer{}
	// Read up to 7 bytes from first word
	firstChunkSize := min(l, 7)
	for i := range firstChunkSize {
		buf.WriteByte(byte(op >> (8 * (i + 1))))
	}
	// Read remaining bytes from subsequent words (8 bytes each)
	remaining := l - firstChunkSize
	wordIdx := addr + 1
	for remaining > 0 {
		op = memory[wordIdx]
		chunkSize := min(remaining, 8)
		for i := range chunkSize {
			buf.WriteByte(byte(op >> (8 * i)))
		}
		remaining -= chunkSize
		wordIdx++
	}
	n, err := out.Write(buf.Bytes())
	if err != nil {
		log.Errf("Failed to output str8: %v", err)
		return -1
	}
	if int64(n) != l {
		log.Errf("Failed to output all bytes: expected %d, got %d", l, n)
		return -1
	}
	return l
}

func executeSyscall(syscall Syscall, operand, accumulator int64,
	memory []Operation, pc ImmediateData,
	isStack bool, stack []Operation, stackPtr int,
) (int64, bool) {
	switch syscall {
	case Exit:
		return operand, true
	case Sleep:
		time.Sleep(time.Duration(operand) * time.Millisecond)
		return accumulator, false
	case Write:
		if isStack {
			addr := stackPtr - int(operand)
			return sysPrint(os.Stdout, stack, addr), false
		}
		addr := int64(pc) + operand
		return sysPrint(os.Stdout, memory, int(addr)), false
	default:
		log.Errf("Unknown syscall: %d", syscall)
	}
	return unknownSyscallAbortCode, true // unknown syscall abort code.
}

const StackSize = 256

//nolint:gocognit,gocyclo,funlen,maintidx // yeah well...
func execute(pc ImmediateData, program []Operation, accumulator int64) (int64, int64) {
	var stack [StackSize]Operation // we use Operation while it's really plain int64 to be compatible when using stack with sysPrint
	stackPtr := -1
	end := ImmediateData(len(program))
	for pc < end {
		op := program[pc]
		switch code := op.Opcode(); code {
		case Sys, SysS:
			arg := op.OperandInt64()
			callID := Syscall(arg & 0xFF) //nolint:gosec // duh... 0xFF means it can't overflow
			v := arg >> 8
			log.Infof("Syscall %v at PC: %d, accumulator: %d - operand: %d (%x)", callID, pc, accumulator, v, v)
			code, abort := executeSyscall(callID, v, accumulator, program, pc, code == SysS, stack[:], stackPtr)
			if abort {
				return accumulator, code
			}
			accumulator = code
		case LoadI:
			accumulator = op.OperandInt64()
			if Debug {
				log.Debugf("LoadI  at PC: %d, value: %d", pc, accumulator)
			}
		case AddI:
			accumulator += op.OperandInt64()
			if Debug {
				log.Debugf("AddI   at PC: %d, value: %d -> %d", pc, op.OperandInt64(), accumulator)
			}
		case SubI:
			accumulator -= op.OperandInt64()
			if Debug {
				log.Debugf("SubI   at PC: %d, value: %d -> %d", pc, op.OperandInt64(), accumulator)
			}
		case MulI:
			accumulator *= op.OperandInt64()
			if Debug {
				log.Debugf("MulI   at PC: %d, value: %d -> %d", pc, op.OperandInt64(), accumulator)
			}
		case DivI:
			accumulator /= op.OperandInt64()
			if Debug {
				log.Debugf("DivI   at PC: %d, value: %d -> %d", pc, op.OperandInt64(), accumulator)
			}
		case ModI:
			accumulator %= op.OperandInt64()
			if Debug {
				log.Debugf("ModI   at PC: %d, value: %d -> %d", pc, op.OperandInt64(), accumulator)
			}
		case ShiftI:
			v := op.OperandInt64()
			if v < 0 {
				accumulator >>= -v
			} else {
				accumulator <<= v
			}
			if Debug {
				log.Debugf("ShiftI at PC: %d, value: %d -> %d", pc, v, accumulator)
			}
		case AndI:
			accumulator &= op.OperandInt64()
			if Debug {
				log.Debugf("AndI   at PC: %d, value: %d -> %d", pc, op.OperandInt64(), accumulator)
			}
		case JNZ:
			if accumulator != 0 {
				if Debug {
					log.Debugf("JNZ    at PC: %d, jumping to PC: +%d", pc, op.OperandInt64())
				}
				pc += op.Operand()
				continue
			}
			if Debug {
				log.Debugf("JNZ    at PC: %d, not jumping", pc)
			}
		case JNEG:
			if accumulator < 0 {
				if Debug {
					log.Debugf("JNEG   at PC: %d, jumping to PC: +%d", pc, op.OperandInt64())
				}
				pc += op.Operand()
				continue
			}
			if Debug {
				log.Debugf("JNEG   at PC: %d, not jumping", pc)
			}
		case JPOS:
			if accumulator >= 0 {
				if Debug {
					log.Debugf("JPOS   at PC: %d, jumping to PC: +%d", pc, op.OperandInt64())
				}
				pc += op.Operand()
				continue
			}
			if Debug {
				log.Debugf("JPOS   at PC: %d, not jumping", pc)
			}
		case JumpR:
			if Debug {
				log.Debugf("JumpR  at PC: %d, jumping to PC: +%d", pc, op.OperandInt64())
			}
			pc += op.Operand()
			continue
		case LoadR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			accumulator = int64(program[pc+offset])
			if Debug {
				log.Debugf("LoadR  at PC: %d, offset: %d, value: %d", pc, offset, accumulator)
			}
		case AddR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			value := int64(program[pc+offset])
			accumulator += value
			if Debug {
				log.Debugf("AddR   at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		case SubR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			value := int64(program[pc+offset])
			accumulator -= value
			if Debug {
				log.Debugf("SubR   at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		case MulR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			value := int64(program[pc+offset])
			accumulator *= value
			if Debug {
				log.Debugf("MulR   at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		case DivR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			value := int64(program[pc+offset])
			accumulator /= value
			if Debug {
				log.Debugf("DivR   at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		case StoreR:
			offset := op.Operand()
			if Debug {
				oldValue := int64(program[pc+offset]) // may panic if offset is out of bounds, that's fine
				log.Debugf("StoreR at PC: %d, offset: %d, old value: %d, new value: %d", pc, offset, oldValue, accumulator)
			}
			// ok to panic if offset is out of bounds
			program[pc+offset] = Operation(accumulator)
		case IncrR:
			arg := op.Operand()
			offset := arg >> 8
			value := int8(arg & 0xff) //nolint:gosec // 0xff implies can't overflow (and we want the sign bit too)
			// ok to panic if offset is out of bounds
			oldValue := int64(program[pc+offset])
			accumulator = oldValue + int64(value)
			program[pc+offset] = Operation(accumulator)
			if Debug {
				log.Debugf("IncrR  at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		// panic / oob in stack access is fine (no checks outside of go's runtime)
		case Call:
			stackPtr++
			stack[stackPtr] = Operation(pc + 1)
			if Debug {
				log.Debugf("Call   at PC: %d, jumping to PC: +%d, SP = %d %v", pc, op.OperandInt64(), stackPtr, stack[:stackPtr+1])
			}
			pc += op.Operand()
			continue
		case Return:
			extra := int(op.OperandInt64())
			if extra > 0 {
				stackPtr -= extra
			}
			oldPC := pc
			pc = ImmediateData(stack[stackPtr])
			stackPtr--
			if Debug {
				log.Debugf("Return  at PC: %d, returning to PC: %d - SP = %d %v", oldPC, pc, stackPtr, stack[:stackPtr+1])
			}
			continue
		case Push:
			for range op.Operand() {
				stackPtr++
				stack[stackPtr] = 0 //nolint:gosec // gosec smoking crack again?
			}
			stackPtr++
			stack[stackPtr] = Operation(accumulator) //nolint:gosec // gosec smoking crack again?
			if Debug {
				log.Debugf("Push   at PC: %d, value: %d - SP = %d %v", pc, accumulator, stackPtr, stack[:stackPtr+1])
			}
		case Pop:
			accumulator = int64(stack[stackPtr])
			stackPtr--
			extra := int(op.OperandInt64())
			if extra > 0 {
				stackPtr -= extra
			}
			if Debug {
				log.Debugf("Pop    at PC: %d, value: %d - SP = %d %v", pc, accumulator, stackPtr, stack[:stackPtr+1])
			}
		case LoadS:
			offset := int(op.Operand())
			accumulator = int64(stack[stackPtr-offset])
			if Debug {
				log.Debugf("LoadS  at PC: %d, offset: %d, value: %d - SP = %d %v", pc, offset, accumulator, stackPtr, stack[:stackPtr+1])
			}
		case StoreS:
			offset := int(op.Operand())
			stack[stackPtr-offset] = Operation(accumulator)
			if Debug {
				log.Debugf("StoreS at PC: %d, offset: %d, value: %d - SP = %d %v", pc, offset, accumulator, stackPtr, stack[:stackPtr+1])
			}
		case AddS:
			offset := int(op.Operand())
			accumulator += int64(stack[stackPtr-offset])
			if Debug {
				log.Debugf("AddS   at PC: %d, offset: %d, value: %d -> %d - SP = %d %v",
					pc, offset, stack[stackPtr-offset], accumulator, stackPtr, stack[:stackPtr+1])
			}
		case IncrS:
			arg := op.Operand()
			offset := int(arg >> 8)
			value := int8(arg & 0xff) //nolint:gosec // 0xff implies can't overflow (and we want the sign bit too)
			if Debug {
				log.Debugf("IncrS  at PC: %d, offset: %d, value: %d - SP = %d %v", pc, offset, value, stackPtr, stack[:stackPtr+1])
			}
			stack[stackPtr-offset] += Operation(value)
			if Debug {
				log.Debugf("IncrS  at PC: %d, offset: %d, value: %d -> %d - SP = %d %v",
					pc, offset, value, stack[stackPtr-offset], stackPtr, stack[:stackPtr+1])
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
