// Package asm provides an assembler for the Grol VM
package asm

import (
	"bufio"
	"encoding/binary"
	"os"
	"strconv"
	"strings"

	"fortio.org/log"
	"grol.io/vm/cpu"
)

type Line struct {
	Op    cpu.Operation
	Label string
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
		reader := bufio.NewScanner(f)
		ret := compile(reader, writer)
		if ret != 0 {
			return ret
		}
		if err := reader.Err(); err != nil {
			log.Errf("Error reading file %s: %v", file, err)
		}
	}
	return 0
}

func parseLine(line string) ([]string, error) {
	var result []string
	var current strings.Builder
	inQuote := false
	prevRune := ' '
	var whichQuote rune
	emit := func() {
		if current.Len() > 0 {
			result = append(result, current.String())
			current.Reset()
		}
	}
	for _, ch := range line {
		switch {
		case inQuote && ch != whichQuote:
			current.WriteRune(ch)
		case !inQuote && (ch == '"' || ch == '\'' || ch == '`'):
			if prevRune != ' ' && prevRune != '\t' {
				return nil, strconv.ErrSyntax
			}
			emit()
			whichQuote = ch
			current.WriteRune(ch)
			inQuote = true
		case inQuote && ch == whichQuote:
			current.WriteRune(ch)
			s, err := strconv.Unquote(current.String())
			if err != nil {
				return nil, err
			}
			result = append(result, s)
			current.Reset()
			inQuote = false
		case ch == '#' && !inQuote:
			emit()
			return result, nil
		case !inQuote && (ch == ' ' || ch == '\t'):
			emit() // collapses all whitespace
		default:
			current.WriteRune(ch)
		}
		prevRune = ch
	}
	if inQuote {
		return nil, strconv.ErrSyntax
	}
	emit()
	return result, nil
}

func sysCalls(op *cpu.Operation, args []string) int {
	syscall, ok := cpu.SyscallFromString(strings.ToLower(args[0]))
	if !ok {
		return log.FErrf("Unknown syscall: %s", args[0])
	}
	v, err := parseArg(args[1])
	if err != nil {
		return log.FErrf("Failed to parse SYS argument %q: %v", args[1], err)
	}
	*op = op.SetOperand(cpu.ImmediateData(v)<<8 | cpu.ImmediateData(syscall))
	return 0
}

func compile(reader *bufio.Scanner, writer *bufio.Writer) int {
	pc := cpu.ImmediateData(0)
	labels := make(map[string]cpu.ImmediateData)
	var result []Line
	for reader.Scan() {
		fields, err := parseLine(reader.Text())
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
			return log.FErrf("Wrong number of arguments for %s, got %d", instr, narg)
		}
		arg := args[0]
		var op cpu.Operation
		label := "" // no label except for instructions that require it
		switch instr {
		case "data":
			// This is using the full 64-bit Operation as data instead of 56+8. There is no instruction.
			v, err := parseArg(arg)
			if err != nil {
				return log.FErrf("Failed to parse data argument %q: %v", arg, err)
			}
			op = cpu.Operation(v)
		default:
			instrEnum, ok := cpu.InstructionFromString(instr)
			if !ok {
				return log.FErrf("Unknown instruction: %s", instr)
			}
			log.Debugf("Parsing instruction: %s %v", instrEnum, args)
			op = op.SetOpcode(instrEnum)
			// Address vs immediate instructions handling
			switch instrEnum {
			case cpu.Sys:
				if failed := sysCalls(&op, args); failed != 0 {
					return failed
				}
			case cpu.JNZ, cpu.LoadR, cpu.AddR, cpu.StoreR:
				// don't parse the argument, it will be resolved later, store the label
				label = arg
			default:
				v, err := parseArg(arg)
				if err != nil {
					return log.FErrf("Failed to parse argument %q: %v", arg, err)
				}
				op = op.SetOperand(cpu.ImmediateData(v))
			}
		}
		result = append(result, Line{Op: op, Label: label})
		pc++
	}
	for pc, line := range result {
		op := line.Op
		switch op.Opcode() {
		case cpu.JNZ, cpu.LoadR, cpu.AddR, cpu.StoreR:
			// resolve label
			targetPC, ok := labels[line.Label]
			if !ok {
				return log.FErrf("Unknown label: %s", line.Label)
			}
			relativePC := targetPC - cpu.ImmediateData(pc)
			op = op.SetOperand(relativePC)
		default:
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
