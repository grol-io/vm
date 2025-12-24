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
		reader := bufio.NewScanner(f)
		pc := cpu.Data(0)
		labels := make(map[string]cpu.Data)
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
			instr := fields[0]
			args := fields[1:]
			instrEnum, ok := cpu.InstructionFromString(instr)
			if !ok {
				return log.FErrf("Unknown instruction: %s", instr)
			}
			log.Debugf("Writing instruction: %s %v", instrEnum, args)
			if len(args) != 1 {
				return log.FErrf("Current instruction %s requires exactly one argument, got %d", instrEnum, len(args))
			}
			arg := args[0]
			op := cpu.Operation{
				Opcode: instrEnum,
			}
			// JNE handling
			switch instrEnum {
			case cpu.JNE:
				// resolve label
				targetPC, ok := labels[arg]
				if !ok {
					return log.FErrf("Unknown label: %s", arg)
				}
				log.Debugf("Resolved JNE label %s to PC: %d", arg, targetPC)
				op.Data = targetPC
			default:
				arg := parseArg(arg)
				op.Data = cpu.Data(arg)
			}
			if err := binary.Write(writer, binary.LittleEndian, op); err != nil {
				return log.FErrf("Failed to write operation: %v", err)
			}
			log.Debugf("Wrote operation: %#+v", op)
			pc++
		}
		if err := reader.Err(); err != nil {
			log.Errf("Error reading file %s: %v", file, err)
		}
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
