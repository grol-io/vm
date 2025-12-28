// Package asm provides an assembler for the Grol VM
package asm

import (
	"bufio"
	"encoding/binary"
	"errors"
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
	for _, file := range files {
		log.Infof("Compiling file: %s", file)
		f, err := os.Open(file)
		if err != nil {
			log.Errf("Failed to open file %s: %v", file, err)
			continue
		}
		defer f.Close()
		// replace .asm with .vm
		outputFile := strings.TrimSuffix(file, ".asm") + ".vm"
		log.Infof("Output file: %s", outputFile)
		out, err := os.Create(outputFile)
		if err != nil {
			log.Errf("Failed to create output file %s: %v", outputFile, err)
			continue
		}
		defer out.Close()
		writer := bufio.NewWriter(out)
		defer writer.Flush()
		_, _ = writer.WriteString(cpu.HEADER)
		reader := bufio.NewReader(f)
		ret := compile(reader, writer)
		if ret != 0 {
			return ret
		}
	}
	return 0
}

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
			result = append(result, s)
			current.Reset()
			inQuote = false
		case ch == '#' && !inQuote:
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

func compile(reader *bufio.Reader, writer *bufio.Writer) int {
	pc := cpu.ImmediateData(0)
	labels := make(map[string]cpu.ImmediateData)
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
		if (narg != 1 && instr != "sys") || (narg != 2 && instr == "sys") {
			return log.FErrf("Wrong number of arguments for %s, got %d (%v)", instr, narg, args)
		}
		arg := args[0]
		var op cpu.Operation
		label := "" // no label except for instructions that require it
		data := true
		is48bit := false
		switch instr {
		case "data":
			// This is using the full 64-bit Operation as data instead of 56+8. There is no instruction.
			v, err := parseArg(arg)
			if err != nil {
				return log.FErrf("Failed to parse data argument %q: %v", arg, err)
			}
			op = cpu.Operation(v)
		case "str8":
			l := len(arg)
			if l == 0 || l > 255 {
				return log.FErrf("str8 argument out of range: %d", l)
			}
			ops := serializeStr8([]byte(arg))
			result = append(result, ops...)
			pc += cpu.ImmediateData(len(ops))
			continue
		default:
			instrEnum, ok := cpu.InstructionFromString(instr)
			if !ok {
				return log.FErrf("Unknown instruction: %s", instr)
			}
			log.Debugf("Parsing instruction: %s %v", instrEnum, args)
			data = false
			op = op.SetOpcode(instrEnum)
			switch instrEnum {
			case cpu.Sys:
				var failed int
				failed, label = sysCalls(&op, args)
				if failed != 0 {
					return failed
				}
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
				return log.FErrf("Unknown label: %s", line.Label)
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
