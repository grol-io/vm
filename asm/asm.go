// Package asm provides an assembler for the Grol VM
package asm

import (
	"bufio"
	"encoding/binary"
	"io"
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
		pc := int64(0)
		labels := make(map[string]int64)
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
			_ = writer.WriteByte(byte(instrEnum)) // error will be caught in EmitInt64
			pc++
			// JNE handling
			if instrEnum == cpu.JNE {
				if len(args) != 1 {
					return log.FErrf("JNE requires exactly one argument")
				}
				// resolve label
				label := args[0]
				targetPC, ok := labels[label]
				if !ok {
					return log.FErrf("Unknown label: %s", label)
				}
				log.Debugf("Resolved JNE label %s to PC: %d", label, targetPC)
				if err := EmitInt64(writer, targetPC); err != nil {
					return log.FErrf("Failed to write argument: %v", err)
				}
				pc += 8
				continue
			}
			for _, arg := range parseArgs(args) {
				if err := EmitInt64(writer, arg); err != nil {
					return log.FErrf("Failed to write argument: %v", err)
				}
				pc += 8
			}
		}
		if err := reader.Err(); err != nil {
			log.Errf("Error reading file %s: %v", file, err)
		}
	}
	return 0
}

func EmitInt64(w io.Writer, val int64) error {
	return binary.Write(w, binary.LittleEndian, val)
}

func parseArgs(args []string) []int64 {
	result := make([]int64, len(args))
	for i, arg := range args {
		var val int64
		val, err := strconv.ParseInt(arg, 0, 64)
		if err != nil {
			log.Errf("Failed to parse argument %q: %v", arg, err)
			continue
		}
		log.Debugf("Parsed argument %q as %d", arg, val)
		result[i] = val
	}
	return result
}
