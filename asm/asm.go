// Package asm provides an assembler for the Grol VM
package asm

import (
	"bufio"
	"fmt"
	"io"
	"os"
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
		for reader.Scan() {
			line := strings.TrimSpace(reader.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				log.Debugf("Skipping line: %s", line)
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
			log.Debugf("Writing arguments: %v", parseArgs(args))
			for _, arg := range parseArgs(args) {
				if err := EmitInt64(writer, arg); err != nil {
					return log.FErrf("Failed to write argument: %v", err)
				}
			}
		}
		if err := reader.Err(); err != nil {
			log.Errf("Error reading file %s: %v", file, err)
		}
	}
	return 0
}

func EmitInt64(w io.Writer, val int64) error {
	b := []byte{
		byte(val >> 56), byte(val >> 48), byte(val >> 40), byte(val >> 32),
		byte(val >> 24), byte(val >> 16), byte(val >> 8), byte(val),
	}
	_, err := w.Write(b)
	return err
}

func parseArgs(args []string) []int64 {
	result := make([]int64, len(args))
	for i, arg := range args {
		var val int64
		_, err := fmt.Sscanf(arg, "%d", &val)
		if err != nil {
			log.Errf("Failed to parse argument %q: %v", arg, err)
			continue
		}
		result[i] = val
	}
	return result
}
