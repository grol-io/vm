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

func compile(reader *bufio.Scanner, writer *bufio.Writer) int {
	pc := cpu.ImmediateData(0)
	labels := make(map[string]cpu.ImmediateData)
	var result []Line
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			log.Debugf("Skipping line: %s", line)
			continue
		}
		// label
		if _, found := strings.CutSuffix(line, ":"); found {
			label := strings.TrimSuffix(line, ":")
			log.Debugf("Found label: %s at PC: %d", label, pc)
			labels[label] = pc
			continue
		}
		fields := strings.Fields(line)
		instr := strings.ToLower(fields[0])
		args := fields[1:]
		if len(args) != 1 {
			return log.FErrf("Currently all instructions (including %s) require exactly one argument, got %d", instr, len(args))
		}
		arg := args[0]
		var op cpu.Operation
		label := "" // no label except for instructions that require it
		switch instr {
		case "data":
			// This is using the full 64-bit Operation as data instead of 56+8. There is no instruction.
			op = cpu.Operation(parseArg(arg))
		default:
			instrEnum, ok := cpu.InstructionFromString(instr)
			if !ok {
				return log.FErrf("Unknown instruction: %s", instr)
			}
			log.Debugf("Parsing instruction: %s %v", instrEnum, args)
			op = op.SetOpcode(instrEnum)
			// Address vs immediate instructions handling
			switch instrEnum {
			case cpu.JNZ, cpu.Load, cpu.Add, cpu.Store:
				// don't parse the argument, it will be resolved later, store the label
				label = arg
			default:
				arg := parseArg(arg)
				op = op.SetOperand(cpu.ImmediateData(arg))
			}
		}
		result = append(result, Line{Op: op, Label: label})
		pc++
	}
	for pc, line := range result {
		op := line.Op
		switch op.Opcode() {
		case cpu.JNZ, cpu.Load, cpu.Add, cpu.Store:
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

func parseArg(arg string) int64 {
	var val int64
	val, err := strconv.ParseInt(arg, 0, 64)
	if err != nil {
		log.Errf("Failed to parse argument %q: %v", arg, err)
		return 0
	}
	log.Debugf("Parsed argument %q as %d", arg, val)
	return val
}
