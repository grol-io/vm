// Package cpu provides the CPU implementation for the Grol VM, everything related to running
// the virtual machine instructions (and defining the instruction set).
package cpu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

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

const StackSize = 600 // for 4k available after argc we need >= 513

type CPU struct {
	Accumulator int64
	PC          ImmediateData
	Program     []Operation
	StackPtr    int
	Stack       [StackSize]Operation // we use Operation while it's really plain int64 to be compatible when using stack with sysPrint
}

const (
	// HEADER for the VM binary format, starts with non printable version byte to indicate it's binary.
	// The first byte is the version byte, followed by the ASCII characters "GROL VM".
	HEADER = "\x01GROL VM"
	// OperationSize is the size of an Operation in bytes (int64).
	OperationSize = 8
)

// so vm run cat | false detects the error instead of silently dying.
func signalSetup() {
	signal.Ignore(syscall.SIGPIPE)
}

func Run(args ...string) int {
	signalSetup()
	cpu := &CPU{}
	rtSize := binary.Size(Operation(0))
	log.Infof("Starting CPU - size of operation: %d bytes", rtSize)
	if rtSize != OperationSize {
		return log.FErrf("Unexpected operation size: got %d, want %d", rtSize, OperationSize)
	}
	if len(args) == 0 {
		return log.FErrf("No file provided to run")
	}
	file := args[0]
	args = args[1:]
	log.Infof("Running file: %s with args: %v", file, args)
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
	cpu.SetArgs(args)
	execResult := cpu.Execute()
	if execResult != 0 {
		log.Warnf("Non 0 exit of program %s: %v", file, execResult)
		return execResult
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

func (c *CPU) SetArgs(args []string) {
	c.StackPtr = 0
	// Each args at the end of program space and address on stack
	argc := len(args)
	for i := range argc {
		arg := args[argc-1-i]
		addr := len(c.Program)
		ops := SerializeStr8([]byte(arg))
		c.Program = append(c.Program, ops...)
		c.Stack[c.StackPtr] = Operation(addr)
		log.LogVf("Set arg %q on stack at %d: %d (length %d)", arg, c.StackPtr, addr, len(ops))
		c.StackPtr++
	}
	// argc last on stack:
	c.Stack[c.StackPtr] = Operation(len(args))
	log.LogVf("Set argc on stack at %d: %d", c.StackPtr, len(args))
}

const unknownSyscallAbortCode = 99

func sysRead(fd int, memory []Operation, addr, n int) int64 {
	if n < 0 {
		panic(fmt.Sprintf("invalid read size: %d", n))
	}
	if n == 0 {
		log.LogVf("Read size is 0, nothing to read")
		return 0
	}
	if len(memory) == 0 {
		panic("memory slice is empty")
	}
	// Cast the memory operations to a byte slice using unsafe
	// Each Operation is an int64, so we need addr*OperationSize bytes offset
	memAsBytes := unsafe.Slice((*byte)(unsafe.Pointer(&memory[0])), len(memory)*OperationSize)
	byteOffset := addr * OperationSize
	// syscall.Read takes []byte
	r, err := syscall.Read(fd, memAsBytes[byteOffset:byteOffset+n])
	if err != nil {
		log.Errf("Failed to read from fd %d: %v", fd, err)
		return -1
	}
	log.LogVf("Read %d bytes from fd %d", r, fd)
	return int64(r)
}

func sysRead8(fd int, memory []Operation, addr, n int) int64 {
	if n <= 0 || n > 255 {
		panic(fmt.Sprintf("invalid read size for str8: %d", n))
	}
	if len(memory) == 0 {
		panic("memory slice is empty")
	}
	// Cast the memory operations to a byte slice using unsafe
	// Each Operation is an int64, so we need addr*OperationSize bytes offset
	memAsBytes := unsafe.Slice((*byte)(unsafe.Pointer(&memory[0])), len(memory)*OperationSize)

	// For str8, the length byte goes at byteOffset 0, data starts at byteOffset 1
	byteOffset := addr * OperationSize

	// syscall.Read takes []byte
	r, err := syscall.Read(fd, memAsBytes[byteOffset+1:byteOffset+1+n])
	if err != nil {
		log.Errf("Failed to read8 from fd %d: %v", fd, err)
		return -1
	}
	log.LogVf("Read8 %d bytes from fd %d", r, fd)
	if r == 0 {
		return 0
	}
	// Set the length byte
	memAsBytes[byteOffset] = byte(r)
	return int64(r)
}

// sysWrite8 writes the str8 bytes and returns the number of bytes it did output.
func sysWrite8(fd int, memory []Operation, addr, offset int) int64 {
	log.LogVf("Writing str8 from memory at addr: %d, offset: %d to fd %d", addr, offset, fd)
	if len(memory) == 0 {
		panic("memory slice is empty")
	}
	// Cast the memory operations to a byte slice using unsafe
	// Each Operation is an int64, so we need addr*OperationSize bytes offset
	memAsBytes := unsafe.Slice((*byte)(unsafe.Pointer(&memory[0])), len(memory)*OperationSize)

	byteOffset := addr*OperationSize + offset
	length := int(memAsBytes[byteOffset])
	if length == 0 {
		return 0
	}
	if log.LogVerbose() {
		// this would alloc a slice so we avoid it unless verbose logging is enabled
		log.LogVf("Before writing bytes: %d %q", length, memAsBytes[byteOffset+1:byteOffset+1+length])
	}
	// Write directly from memory without copying
	n, err := syscall.Write(fd, memAsBytes[byteOffset+1:byteOffset+1+length])
	log.LogVf("Wrote %d bytes to fd %d (err %v)", n, fd, err)

	if err != nil {
		log.Errf("Failed to output str8: %v", err)
		return -1
	}
	if n != length {
		log.Errf("Failed to output all bytes: expected %d, got %d", length, n)
		return -1
	}
	return int64(length)
}

// sysOpen opens a file and returns the file descriptor.
// It reads the filename (str8) from memory at the given address.
func sysOpen(memory []Operation, addr int) int64 {
	log.LogVf("Opening file, path at memory addr: %d", addr)
	if len(memory) == 0 {
		panic("memory slice is empty")
	}
	memAsBytes := unsafe.Slice((*byte)(unsafe.Pointer(&memory[0])), len(memory)*OperationSize)
	byteOffset := addr * OperationSize
	// Read str8 length
	length := int(memAsBytes[byteOffset])
	if length == 0 {
		log.Errf("sysOpen: empty filename")
		return -1
	}
	// Extract filename
	filename := string(memAsBytes[byteOffset+1 : byteOffset+1+length])
	log.LogVf("Opening file: %q", filename)

	// syscall.Open returns (int, error)
	// We use O_RDONLY for now as per requirements
	fd, err := syscall.Open(filename, syscall.O_RDONLY, 0)
	if err != nil {
		log.Errf("Failed to open file %q: %v", filename, err)
		return -1
	}
	log.LogVf("Opened file %q, fd: %d", filename, fd)
	return int64(fd)
}

// sysWrite writes the n bytes and returns the number of bytes it did output.
func sysWrite(fd int, memory []Operation, addr, n int) int64 {
	log.LogVf("Writing n bytes from memory at addr: %d, n: %d to fd %d", addr, n, fd)
	if n < 0 {
		panic(fmt.Sprintf("invalid write size: %d", n))
	}
	if n == 0 {
		return 0
	}
	if len(memory) == 0 {
		panic("memory slice is empty")
	}
	// Cast the memory operations to a byte slice using unsafe
	// Each Operation is an int64, so we need addr*OperationSize bytes offset
	memAsBytes := unsafe.Slice((*byte)(unsafe.Pointer(&memory[0])), len(memory)*OperationSize)
	byteOffset := addr * OperationSize
	if log.LogVerbose() {
		// this would alloc a slice so we avoid it unless verbose logging is enabled
		log.LogVf("Before writing bytes: %d %q", n, memAsBytes[byteOffset:byteOffset+n])
	}
	// Write directly from memory without copying
	m, err := syscall.Write(fd, memAsBytes[byteOffset:byteOffset+n])
	log.LogVf("Wrote %d bytes to fd %d (err %v)", m, fd, err)
	if err != nil {
		log.Errf("Failed to output bytes: %v", err)
		return -1
	}
	if n != m {
		log.Errf("Failed to output all bytes: expected %d, got %d", n, m)
		return -1
	}
	return int64(n)
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
	case Read8:
		if isStack {
			addr := stackPtr - int(operand)
			return sysRead8(0, stack, addr, int(accumulator)), false
		}
		addr := int64(pc) + operand
		return sysRead8(0, memory, int(addr), int(accumulator)), false
	case Write8:
		if isStack {
			addr := stackPtr - int(operand) + int(accumulator)/8
			return sysWrite8(1, stack, addr, int(accumulator%8)), false
		}
		addr := int64(pc) + operand
		if operand == 0 { // 0 means use accumulator as address
			addr = accumulator
		}
		return sysWrite8(1, memory, int(addr), 0), false
	case ReadN:
		if isStack {
			addr := stackPtr - int(operand)
			return sysRead(0, stack, addr, int(accumulator)), false
		}
		addr := int64(pc) + operand
		return sysRead(0, memory, int(addr), int(accumulator)), false
	case WriteN:
		if isStack {
			addr := stackPtr - int(operand)
			return sysWrite(1, stack, addr, int(accumulator)), false
		}
		addr := int64(pc) + operand
		return sysWrite(1, memory, int(addr), int(accumulator)), false
	case Read, Write:
		return executePackedSyscall(syscall, operand, accumulator, memory, pc, isStack)
	default:
		log.Errf("Unknown syscall: %d", syscall)
	}
	return unknownSyscallAbortCode, true // unknown syscall abort code.
}

func splitSigned8bits(v int64) (int64, int64) {
	return v >> 8, int64(int8(v & 0xFF)) //nolint:gosec // 0xFF implies can't overflow and we want the sign bit
}

func splitUnsigned8bits(v int64) (int64, int64) {
	return v >> 8, int64(uint8(v & 0xFF)) //nolint:gosec // 0xFF implies can't overflow
}

//nolint:gocognit,gocyclo,funlen,maintidx // yeah well...
func execute(pc ImmediateData, program []Operation, accumulator int64, stack []Operation, stackPtr int) (int64, int64) {
	end := ImmediateData(len(program))
	for pc < end {
		op := program[pc]
		switch code := op.Opcode(); code {
		case Sys, SysS:
			v, cid := splitUnsigned8bits(op.OperandInt64())
			callID := Syscall(cid) //nolint:gosec // 0xFF in splitUnsigned8bits means it can't overflow
			log.Infof("Syscall %v at PC: %d, accumulator: %d - operand: %d (%x)", callID, pc, accumulator, v, v)
			code, abort := executeSyscall(callID, v, accumulator, program, pc, code == SysS, stack, stackPtr)
			if abort {
				return accumulator, code
			}
			accumulator = code
		case LoadI:
			accumulator = op.OperandInt64()
			if Debug {
				log.Debugf("LoadI   at PC: %d, value: %d", pc, accumulator)
			}
		case AddI:
			accumulator += op.OperandInt64()
			if Debug {
				log.Debugf("AddI    at PC: %d, value: %d -> %d (%x)", pc, op.OperandInt64(), accumulator, accumulator)
			}
		case SubI:
			accumulator -= op.OperandInt64()
			if Debug {
				log.Debugf("SubI    at PC: %d, value: %d -> %d (%x)", pc, op.OperandInt64(), accumulator, accumulator)
			}
		case MulI:
			accumulator *= op.OperandInt64()
			if Debug {
				log.Debugf("MulI    at PC: %d, value: %d -> %d (%x)", pc, op.OperandInt64(), accumulator, accumulator)
			}
		case DivI:
			accumulator /= op.OperandInt64()
			if Debug {
				log.Debugf("DivI    at PC: %d, value: %d -> %d (%x)", pc, op.OperandInt64(), accumulator, accumulator)
			}
		case ModI:
			accumulator %= op.OperandInt64()
			if Debug {
				log.Debugf("ModI    at PC: %d, value: %d -> %d (%x)", pc, op.OperandInt64(), accumulator, accumulator)
			}
		case ShiftI:
			v := op.OperandInt64()
			if v < 0 {
				accumulator >>= -v
			} else {
				accumulator <<= v
			}
			if Debug {
				log.Debugf("ShiftI  at PC: %d, value: %d -> %d", pc, v, accumulator)
			}
		case AndI:
			accumulator &= op.OperandInt64()
			if Debug {
				log.Debugf("AndI    at PC: %d, value: %d -> %d", pc, op.OperandInt64(), accumulator)
			}
		case JNE:
			param := op.OperandInt64()
			addr, value := splitSigned8bits(param)
			if accumulator != value {
				if Debug {
					log.Debugf("JNE     at PC: %d, jumping to PC: +%d", pc, addr)
				}
				pc += ImmediateData(addr)
				continue
			}
			if Debug {
				log.Debugf("JNE     at PC: %d, not jumping", pc)
			}
		case JEQ:
			param := op.OperandInt64()
			addr, value := splitSigned8bits(param)
			if accumulator == value {
				if Debug {
					log.Debugf("JEQ     at PC: %d, jumping to PC: +%d", pc, addr)
				}
				pc += ImmediateData(addr)
				continue
			}
			if Debug {
				log.Debugf("JEQ     at PC: %d, not jumping", pc)
			}
		case JLT:
			param := op.OperandInt64()
			addr, value := splitSigned8bits(param)
			if accumulator < value {
				if Debug {
					log.Debugf("JLT     at PC: %d, jumping to PC: +%d", pc, addr)
				}
				pc += ImmediateData(addr)
				continue
			}
			if Debug {
				log.Debugf("JLT     at PC: %d, not jumping", pc)
			}
		case JGT:
			param := op.OperandInt64()
			addr, value := splitSigned8bits(param)
			if accumulator > value {
				if Debug {
					log.Debugf("JGT     at PC: %d, jumping to PC: +%d", pc, addr)
				}
				pc += ImmediateData(addr)
				continue
			}
			if Debug {
				log.Debugf("JGT     at PC: %d, not jumping", pc)
			}
		case JGTE:
			param := op.OperandInt64()
			addr, value := splitSigned8bits(param)
			if accumulator >= value {
				if Debug {
					log.Debugf("JGTE    at PC: %d, jumping to PC: +%d", pc, addr)
				}
				pc += ImmediateData(addr)
				continue
			}
			if Debug {
				log.Debugf("JGTE    at PC: %d, not jumping", pc)
			}
		case JLTE:
			param := op.OperandInt64()
			addr, value := splitSigned8bits(param)
			if accumulator <= value {
				if Debug {
					log.Debugf("JLTE    at PC: %d, jumping to PC: +%d", pc, addr)
				}
				pc += ImmediateData(addr)
				continue
			}
			if Debug {
				log.Debugf("JLTE    at PC: %d, not jumping", pc)
			}
		case JumpR:
			if Debug {
				log.Debugf("JumpR   at PC: %d, jumping to PC: +%d", pc, op.OperandInt64())
			}
			pc += op.Operand()
			continue
		case LoadR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			accumulator = int64(program[pc+offset])
			if Debug {
				log.Debugf("LoadR   at PC: %d, offset: %d, value: %d", pc, offset, accumulator)
			}
		case AddR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			value := int64(program[pc+offset])
			accumulator += value
			if Debug {
				log.Debugf("AddR    at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		case SubR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			value := int64(program[pc+offset])
			accumulator -= value
			if Debug {
				log.Debugf("SubR    at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		case MulR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			value := int64(program[pc+offset])
			accumulator *= value
			if Debug {
				log.Debugf("MulR    at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		case DivR:
			offset := op.Operand()
			// ok to panic if offset is out of bounds
			value := int64(program[pc+offset])
			accumulator /= value
			if Debug {
				log.Debugf("DivR    at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		case StoreR:
			offset := op.Operand()
			if Debug {
				oldValue := int64(program[pc+offset]) // may panic if offset is out of bounds, that's fine
				log.Debugf("StoreR  at PC: %d, offset: %d, old value: %d, new value: %d", pc, offset, oldValue, accumulator)
			}
			// ok to panic if offset is out of bounds
			program[pc+offset] = Operation(accumulator)
		case IncrR:
			offset, value := splitSigned8bits(op.OperandInt64())
			addr := pc + ImmediateData(offset)
			// ok to panic if offset is out of bounds
			oldValue := int64(program[addr])
			accumulator = oldValue + value
			program[addr] = Operation(accumulator)
			if Debug {
				log.Debugf("IncrR   at PC: %d, offset: %d, value: %d -> %d", pc, offset, value, accumulator)
			}
		// panic / oob in stack access is fine (no checks outside of go's runtime)
		case Call:
			stackPtr++
			stack[stackPtr] = Operation(pc + 1)
			if Debug {
				log.Debugf("Call    at PC: %d, jumping to PC: +%d, SP = %d %v", pc, op.OperandInt64(), stackPtr, stack[:stackPtr+1])
			}
			pc += op.Operand()
			continue
		case Ret:
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
				stack[stackPtr] = 0
			}
			stackPtr++
			stack[stackPtr] = Operation(accumulator)
			if Debug {
				log.Debugf("Push    at PC: %d, value: %d - SP = %d %v", pc, accumulator, stackPtr, stack[:stackPtr+1])
			}
		case Pop:
			accumulator = int64(stack[stackPtr])
			stackPtr--
			extra := int(op.OperandInt64())
			if extra > 0 {
				stackPtr -= extra
			}
			if Debug {
				log.Debugf("Pop     at PC: %d, value: %d - SP = %d %v", pc, accumulator, stackPtr, stack[:stackPtr+1])
			}
		case LoadS:
			offset := int(op.Operand())
			accumulator = int64(stack[stackPtr-offset])
			if Debug {
				log.Debugf("LoadS   at PC: %d, offset: %d, value: %d - SP = %d %v", pc, offset, accumulator, stackPtr, stack[:stackPtr+1])
			}
		case StoreS:
			offset := int(op.Operand())
			stack[stackPtr-offset] = Operation(accumulator)
			if Debug {
				log.Debugf("StoreS  at PC: %d, offset: %d, value: %d - SP = %d %v", pc, offset, accumulator, stackPtr, stack[:stackPtr+1])
			}
		case AddS:
			offset := int(op.Operand())
			accumulator += int64(stack[stackPtr-offset])
			if Debug {
				log.Debugf("AddS    at PC: %d, offset: %d, value: %d -> %d - SP = %d %v",
					pc, offset, stack[stackPtr-offset], accumulator, stackPtr, stack[:stackPtr+1])
			}
		case SubS:
			offset := int(op.Operand())
			accumulator -= int64(stack[stackPtr-offset])
			if Debug {
				log.Debugf("SubS    at PC: %d, offset: %d, value: %d -> %d - SP = %d %v",
					pc, offset, stack[stackPtr-offset], accumulator, stackPtr, stack[:stackPtr+1])
			}
		case MulS:
			offset := int(op.Operand())
			accumulator *= int64(stack[stackPtr-offset])
			if Debug {
				log.Debugf("MulS    at PC: %d, offset: %d, value: %d -> %d - SP = %d %v",
					pc, offset, stack[stackPtr-offset], accumulator, stackPtr, stack[:stackPtr+1])
			}
		case DivS:
			offset := int(op.Operand())
			accumulator /= int64(stack[stackPtr-offset])
			if Debug {
				log.Debugf("DivS    at PC: %d, offset: %d, value: %d -> %d - SP = %d %v",
					pc, offset, stack[stackPtr-offset], accumulator, stackPtr, stack[:stackPtr+1])
			}
		case IncrS:
			offset, value := splitSigned8bits(op.OperandInt64())
			addr := stackPtr - int(offset)
			oldValue := stack[addr]
			accumulator = int64(oldValue) + value
			stack[addr] = Operation(accumulator)
			if Debug {
				log.Debugf("IncrS   at PC: %d, offset: %d, value: %d -> %d - SP = %d %v",
					pc, offset, value, accumulator, stackPtr, stack[:stackPtr+1])
			}
		case IdivS:
			offset := int(op.Operand())
			current := int64(stack[stackPtr-offset])
			stack[stackPtr-offset] = Operation(current / accumulator)
			accumulator = current % accumulator
			if Debug {
				log.Debugf("IdivS   at PC: %d, offset: %d, value: %d -> %d, remainder: %d - SP = %d %v",
					pc, offset, current, stack[stackPtr-offset], accumulator, stackPtr, stack[:stackPtr+1])
			}
		case LoadSB:
			offset, bytesStackIndex := splitUnsigned8bits(op.OperandInt64())
			bytesOffset := int(stack[stackPtr-int(bytesStackIndex)])
			wordOffset := bytesOffset / 8
			value := stack[stackPtr-int(offset)+wordOffset]
			innerOffsetBits := uint((bytesOffset % 8) * 8)                 //nolint:gosec // (bytesOffset % 8) is always 0-7, so *8 is 0-56
			accumulator = int64((uint64(value) >> innerOffsetBits) & 0xff) //nolint:gosec // shift amount is always 0-56
			if Debug {
				log.Debugf("LoadSB at PC: %d, baseOffset: %d, bytesStackIndex: %d, bytesOffset: %d,"+
					" value: %x -> accumulator: %x - SP = %d",
					pc, offset, bytesStackIndex, bytesOffset, value, accumulator, stackPtr)
			}
		case StoreSB:
			offset, bytesStackIndex := splitUnsigned8bits(op.OperandInt64())
			bytesOffset := int(stack[stackPtr-int(bytesStackIndex)])
			wordOffset := bytesOffset / 8
			oldValue := stack[stackPtr-int(offset)+wordOffset]
			innerOffsetBits := uint((bytesOffset % 8) * 8) //nolint:gosec // (bytesOffset % 8) is always 0-7, so *8 is 0-56
			newValue := (oldValue & ^(0xff << innerOffsetBits)) | (Operation(accumulator&0xff) << innerOffsetBits)
			stack[stackPtr-int(offset)+wordOffset] = newValue
			if Debug {
				log.Debugf("StoreSB at PC: %d, baseOffset: %d, bytesStackIndex: %d, bytesOffset: %d,"+
					" oldValue: %x -> newValue: %x - SP = %d %x",
					pc, offset, bytesStackIndex, bytesOffset, oldValue, newValue, stackPtr, stack[:stackPtr+1])
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
	accumulator, exitCode := execute(c.PC, c.Program, c.Accumulator, c.Stack[:], c.StackPtr)
	c.Accumulator = accumulator
	return int(exitCode)
}
