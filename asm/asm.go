// Package asm provides an assembler for the Grol VM
package asm

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"fortio.org/log"
	"grol.io/vm/cpu"
)

type Line struct {
	Op      cpu.Operation
	Label   string
	Data    bool
	Is48bit bool
}

func Compile(files ...string) int {
	readers := make([]io.Reader, 0, len(files))
	var writer *bufio.Writer
	for i, file := range files {
		log.Infof("Compiling file: %s", file)
		f, err := os.Open(file)
		if err != nil {
			return log.FErrf("Failed to open file %s: %v", file, err)
		}
		defer f.Close()
		// replace .asm with .vm
		if !strings.HasSuffix(file, ".asm") {
			return log.FErrf("Invalid file extension for %s, expected .asm", file)
		}
		if i == 0 {
			outputFile := strings.TrimSuffix(file, ".asm") + ".vm"
			log.Infof("Output file: %s", outputFile)
			out, err := os.Create(outputFile)
			if err != nil {
				return log.FErrf("Failed to create output file %s: %v", outputFile, err)
			}
			defer out.Close()
			writer = bufio.NewWriter(out)
			defer writer.Flush()
			_, _ = writer.WriteString(cpu.HEADER)
		}
		readers = append(readers, f)
	}
	reader := bufio.NewReader(io.MultiReader(readers...))
	return compile(reader, writer)
}

//nolint:gocyclo // it's a full parser.
func parse(reader *bufio.Reader) ([]string, error) {
	var result []string
	var current strings.Builder
	inQuote := false
	inEscape := false
	prevRune := ' '
	var whichQuote rune
	emit := func() {
		if current.Len() > 0 {
			result = append(result, current.String())
			current.Reset()
		}
	}
	var err error
	var ch rune
loop:
	for {
		ch, _, err = reader.ReadRune()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch {
		case ch == '\r':
			continue // just ignore all CRs (windows extra line terminator)
		case ch == '\n' && (!inQuote || whichQuote != '`'):
			break loop
		case !inQuote && (ch == '"' || ch == '\'' || ch == '`'):
			if prevRune != ' ' && prevRune != '\t' {
				log.Errf("Unexpected quote %q in the middle of a token; current token so far: %q", ch, current.String())
				return nil, strconv.ErrSyntax
			}
			emit()
			whichQuote = ch
			current.WriteRune(ch)
			inQuote = true
		case inQuote && ch == whichQuote && !inEscape:
			current.WriteRune(ch)
			s, errUnquote := strconv.Unquote(current.String())
			if errUnquote != nil {
				return nil, errUnquote
			}
			if whichQuote == '\'' {
				// get the rune value
				r := []rune(s)[0]
				result = append(result, fmt.Sprintf("0x%x", r))
			} else {
				result = append(result, s)
			}
			current.Reset()
			inQuote = false
		case (ch == '#' || ch == ';') && !inQuote:
			emit()
			// skip the rest of the line as a comment
			_, _ = reader.ReadString('\n')
			return result, nil
		case !inQuote && (ch == ' ' || ch == '\t'):
			emit() // collapses all whitespace
		case !inEscape && ch == '\\' && inQuote && whichQuote != '`':
			current.WriteRune(ch)
			inEscape = true
		default:
			current.WriteRune(ch)
			inEscape = false
		}
		prevRune = ch
	}
	if inQuote {
		log.Errf("Unterminated quote %c at the end of line/file; started with: %q", whichQuote, current.String())
		return nil, strconv.ErrSyntax
	}
	emit()
	if len(result) != 0 {
		err = nil
	}
	return result, err
}

func isAddressLabel(s string) bool {
	return unicode.IsLetter(rune(s[0]))
}

func sysCalls(op *cpu.Operation, args []string) (int, string) {
	sysCallStr := args[0]
	arg := args[1]
	noLabel := ""
	syscall, ok := cpu.SyscallFromString(strings.ToLower(sysCallStr))
	if !ok {
		return log.FErrf("Unknown syscall: %s", sysCallStr), noLabel
	}
	if isAddressLabel(arg) {
		*op = op.SetOperand(cpu.ImmediateData(syscall))
		return 0, arg
	}
	v, err := parseArg(arg)
	if err != nil {
		return log.FErrf("Failed to parse SYS argument %q: %v", arg, err), noLabel
	}
	// check if the argument is within the valid range for a syscall operand - 48 bits are left
	// so signed range is -(1<<47) to (1<<47)-1
	if v > (1<<47)-1 || v < -(1<<47) {
		return log.FErrf("SYS argument %q out of range: %d %x vs %d", arg, v, v, (1 << 47)), noLabel
	}
	*op = op.SetOperand(cpu.ImmediateData(v)<<8 | cpu.ImmediateData(syscall))
	return 0, noLabel
}

// serialize serializes numbytes (<= 8) bytes of data into 1 int64.
func serialize(b []byte) cpu.Operation {
	if len(b) == 0 || len(b) > 8 {
		panic("unsupported number of bytes")
	}
	var result uint64
	for i := len(b) - 1; i >= 0; i-- {
		result <<= 8
		result |= uint64(b[i])
	}
	return cpu.Operation(result) //nolint:gosec // no overflow, just bits shoving unsigned to signed.
}

func serializeStr8(b []byte) []Line {
	l := len(b)
	if l == 0 || l > 255 {
		panic("str8 can only handle strings 1-255 bytes")
	}
	var result []Line
	// First word: up to 7 bytes of data + length byte
	firstChunkSize := min(l, 7)
	result = append(result, Line{
		Op:   serialize(b[:firstChunkSize])<<8 | cpu.Operation(l),
		Data: true,
	})
	// Remaining bytes in chunks of 8
	remaining := b[firstChunkSize:]
	for len(remaining) > 0 {
		chunkSize := min(len(remaining), 8)
		result = append(result, Line{
			Op:   serialize(remaining[:chunkSize]),
			Data: true,
		})
		remaining = remaining[chunkSize:]
	}
	return result
}

//nolint:gocognit,funlen,gocyclo,maintidx // yes it is a full assembler...
func compile(reader *bufio.Reader, writer *bufio.Writer) int {
	pc := cpu.ImmediateData(0)
	labels := make(map[string]cpu.ImmediateData)
	varmap := make(map[string]cpu.ImmediateData)
	returnN := 0
	var result []Line
	for {
		fields, err := parse(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return log.FErrf("Failed to parse line: %v", err)
		}
		if len(fields) == 0 {
			continue
		}
		first := fields[0]
		// label
		if _, found := strings.CutSuffix(first, ":"); found {
			label := strings.TrimSuffix(first, ":")
			log.Debugf("Found label: %s at PC: %d", label, pc)
			labels[label] = pc
			continue
		}
		instr := strings.ToLower(first)
		args := fields[1:]
		narg := len(args)
		switch instr {
		case "return":
			if narg != 0 {
				return log.FErrf("Expecting 0 arguments for return, got %d (%v)", narg, args)
			}
		case "var", "param":
			if narg == 0 {
				return log.FErrf("Expecting at least 1 argument for %s, got none", instr)
			}
		case "incrr", "incrs", "sys", "syss", "storesb":
			if narg != 2 {
				return log.FErrf("Expecting 2 arguments for %s, got %d (%v)", instr, narg, args)
			}
		default:
			if narg != 1 {
				return log.FErrf("Expecting 1 argument for %s, got %d (%v)", instr, narg, args)
			}
		}
		var op cpu.Operation
		label := "" // no label except for instructions that require it
		data := true
		is48bit := false
		switch instr {
		case "data":
			// This is using the full 64-bit Operation as data instead of 56+8. There is no instruction.
			v, err := parseArg(args[0])
			if err != nil {
				return log.FErrf("Failed to parse data argument %q: %v", args[0], err)
			}
			op = cpu.Operation(v)
		case "str8":
			l := len(args[0])
			if l == 0 || l > 255 {
				return log.FErrf("str8 argument out of range: %d", l)
			}
			ops := serializeStr8([]byte(args[0]))
			result = append(result, ops...)
			pc += cpu.ImmediateData(len(ops))
			continue
		case "var":
			data = false
			clear(varmap)
			op = op.SetOpcode(cpu.Push)
			op = op.SetOperand(cpu.ImmediateData(narg - 1))
			returnN = narg
			for i := range narg {
				varmap[args[i]] = cpu.ImmediateData(i)
			}
			log.Debugf("Var -> Push %d and defined variables: %v", narg-1, varmap)
		case "param":
			// define more stack labels
			start := returnN + 1 // +1 to skip over the return PC
			for i := range narg {
				varmap[args[i]] = cpu.ImmediateData(start + i)
			}
			log.Debugf("Param -> Defined parameters: %v", varmap)
			continue
		case "return":
			data = false
			op = op.SetOpcode(cpu.Ret)
			op = op.SetOperand(cpu.ImmediateData(returnN))
			log.Debugf("Return -> Ret %d", returnN)
			// Don't reset returnN or varmap because there could be more than 1 return
			// point.
		default:
			instrEnum, ok := cpu.InstructionFromString(instr)
			if !ok {
				return log.FErrf("Unknown instruction: %s", instr)
			}
			log.Debugf("Parsing instruction: %s %v", instrEnum, args)
			if instrEnum >= cpu.LoadS { // for stack instructions, resolve var references
				for i, v := range args {
					if !isAddressLabel(v) {
						continue
					}
					if idx, ok := varmap[v]; ok {
						log.Debugf("Resolved var %s to index %d", v, idx)
						args[i] = fmt.Sprintf("%d", idx)
					} else if instrEnum != cpu.SysS || i != 0 {
						// First argument of SysS is the syscall name not a stack variable.
						return log.FErrf("Unknown stack variable: %s", v)
					}
				}
			}
			arg := args[0]
			data = false
			op = op.SetOpcode(instrEnum)
			switch instrEnum {
			case cpu.Sys, cpu.SysS:
				var failed int
				failed, label = sysCalls(&op, args)
				if failed != 0 {
					return failed
				}
				is48bit = true
			case cpu.StoreSB:
				// Store byte at stack index (first argument) with byte offset from stack index (second argument)
				v1, err := parseArg(args[0])
				if err != nil {
					return log.FErrf("Failed to parse argument %q: %v", args[0], err)
				}
				if v1 < 0 || v1 >= cpu.StackSize {
					return log.FErrf("StoreSB stack base out of range (0 to %d): %d", cpu.StackSize-1, v1)
				}
				v2, err := parseArg(args[1])
				if err != nil {
					return log.FErrf("Failed to parse stack index argument %q: %v", args[1], err)
				}
				if v2 < 0 || v2 >= cpu.StackSize {
					return log.FErrf("StoreSB byte offset stack index out of range (0 to %d): %d", cpu.StackSize-1, v2)
				}
				op = op.SetOperand(cpu.ImmediateData(v2))
				op = op.Set48BitsOperand(cpu.ImmediateData(v1))
				is48bit = true
			case cpu.IncrS:
				// Increment by delta (first argument) at stack index (second argument)
				v1, err := parseArg(args[0])
				if err != nil {
					return log.FErrf("Failed to parse argument %q: %v", args[0], err)
				}
				if v1 < -128 || v1 > 127 {
					return log.FErrf("IncrS immediate value out of range (-128 to 127): %d", v1)
				}
				v2, err := parseArg(args[1])
				if err != nil {
					return log.FErrf("Failed to parse stack index argument %q: %v", args[1], err)
				}
				if v2 < 0 || v2 >= cpu.StackSize {
					return log.FErrf("IncrS stack index out of range (0 to %d): %d", cpu.StackSize-1, v2)
				}
				op = op.SetOperand(cpu.ImmediateData(v1))
				op = op.Set48BitsOperand(cpu.ImmediateData(v2))
				is48bit = true
			case cpu.IncrR:
				// 2 arguments: value (-128 to 127) and label
				label = args[1]
				v, err := parseArg(args[0])
				if err != nil {
					return log.FErrf("Failed to parse argument %q: %v", args[0], err)
				}
				if v < -128 || v > 127 {
					return log.FErrf("IncrR immediate value out of range (-128 to 127): %d", v)
				}
				op = op.SetOperand(cpu.ImmediateData(v))
				is48bit = true
			default:
				// allow labels as arguments even for immediate operands (eg load the address into accumulator)
				if isAddressLabel(arg) {
					label = arg
					break
				}
				v, err := parseArg(arg)
				if err != nil {
					return log.FErrf("Failed to parse argument %q: %v", arg, err)
				}
				op = op.SetOperand(cpu.ImmediateData(v))
			}
		}
		result = append(result, Line{Op: op, Label: label, Data: data, Is48bit: is48bit})
		pc++
	}
	return emitCode(writer, result, labels)
}

func emitCode(writer io.Writer, result []Line, labels map[string]cpu.ImmediateData) int {
	for pc, line := range result {
		op := line.Op
		if !line.Data && line.Label != "" {
			// resolve label
			targetPC, ok := labels[line.Label]
			if !ok {
				return log.FErrf("Unknown label: %s for %#v", line.Label, line)
			}
			relativePC := targetPC - cpu.ImmediateData(pc)
			if line.Is48bit {
				op = op.Set48BitsOperand(relativePC)
			} else {
				op = op.SetOperand(relativePC)
			}
		}
		if err := binary.Write(writer, binary.LittleEndian, op); err != nil {
			return log.FErrf("Failed to write operation: %v", err)
		}
		log.Debugf("Wrote operation: %x %v %v", (uint64)(op), op.Opcode(), op.Operand()) //nolint:gosec // on purpose
	}
	return 0
}

func parseArg(arg string) (int64, error) {
	var val int64
	val, err := strconv.ParseInt(arg, 0, 64)
	if err != nil {
		return 0, err
	}
	log.Debugf("Parsed argument %q as %d", arg, val)
	return val, nil
}
