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
	Op            cpu.Operation
	Label         string
	Data          bool
	remainingBits int // How many bits remain for the final operand/address (56 = full, 48 = one 8-bit packed, 40 = two 8-bit packed, etc.)
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

type Resolver struct {
	labels map[string]cpu.ImmediateData
	vars   map[string]cpu.ImmediateData
	consts map[string]int64
}

func NewResolver() *Resolver {
	return &Resolver{
		labels: make(map[string]cpu.ImmediateData),
		vars:   make(map[string]cpu.ImmediateData),
		consts: make(map[string]int64),
	}
}

func (r *Resolver) AddLabel(label string, value cpu.ImmediateData) {
	r.labels[label] = value
}

func (r *Resolver) ClearVars() {
	clear(r.vars)
}

func (r *Resolver) AddVar(name string, value cpu.ImmediateData) {
	r.vars[name] = value
}

func (r *Resolver) AddConst(name string, value int64) error {
	if oldV, ok := r.consts[name]; ok {
		if oldV == value {
			return nil // noop
		}
		return fmt.Errorf("trying to change %q from %d to %d", name, oldV, value)
	}
	r.consts[name] = value
	return nil
}

func (r *Resolver) Labels(name string) (cpu.ImmediateData, bool) {
	v, ok := r.labels[name]
	return v, ok
}

func (r *Resolver) Var(name string) (cpu.ImmediateData, bool) {
	v, ok := r.vars[name]
	return v, ok
}

func (r *Resolver) Const(name string) (int64, bool) {
	v, ok := r.consts[name]
	return v, ok
}

func (r *Resolver) ResValue(strVal string) (int64, error) {
	if v, ok := r.Const(strVal); ok {
		return v, nil
	}
	// Expression ? just basic operator
	if op, left, right := OperatorSplit(strVal); op != "" {
		return r.ResOperator(op, left, right)
	}
	return parseArg(strVal)
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
	v, err := parseArg(arg)
	if err != nil {
		if isAddressLabel(arg) {
			*op = op.SetOperand(cpu.ImmediateData(syscall))
			return 0, arg
		}
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

func serializeStr8(b []byte) []Line {
	ops := cpu.SerializeStr8(b)
	result := make([]Line, 0, len(ops))
	for _, op := range ops {
		result = append(result, Line{
			Op:   op,
			Data: true,
		})
	}
	return result
}

//nolint:gocognit,funlen,gocyclo,maintidx // yes it is a full assembler...
func compile(reader *bufio.Reader, writer *bufio.Writer) int {
	pc := cpu.ImmediateData(0)
	resolver := NewResolver()
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
			resolver.AddLabel(label, pc)
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
		case ".const", "incrr", "incrs", "sys", "syss", "loadsb", "storesb", "jne", "jeq", "jlt", "jgt", "jgte", "jlte":
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
		remainingBits := 56 // default: full 56-bit operand (8-bit opcode + 56-bit operand)
		switch instr {
		case ".const":
			v, err := resolver.ResValue(args[1]) // allow const to alias other consts defined before
			if err != nil {
				return log.FErrf("Failed to parse .const value %q for %q: %v", args[1], args[0], err)
			}
			err = resolver.AddConst(args[0], v)
			if err != nil {
				return log.FErrf(".const: %v", err)
			}
			continue
		case ".space":
			// reserve multiple 0 initialized words
			count, err := resolver.ResValue(args[0])
			if err != nil {
				return log.FErrf("Failed to parse .space argument %q: %v", args[0], err)
			}
			if count <= 0 {
				return log.FErrf(".space argument must be positive, got %d", count)
			}
			for range count {
				result = append(result, Line{
					Op:   cpu.Operation(0),
					Data: true,
				})
			}
			pc += cpu.ImmediateData(count)
			continue
		case "data":
			// This is using the full 64-bit Operation as data instead of 56+8. There is no instruction.
			v, err := resolver.ResValue(args[0])
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
			resolver.ClearVars()
			var totalWords int64
			for _, arg := range args {
				count := int64(1)
				if idx := strings.Index(arg, "["); idx >= 0 {
					// Parse var[N] syntax: reserve N slots with label pointing to the last
					varName := arg[:idx]
					countStr := strings.TrimSuffix(arg[idx+1:], "]")
					var err error
					count, err = resolver.ResValue(countStr)
					if err != nil {
						return log.FErrf("Failed to parse var array size %q: %v", countStr, err)
					}
					if count <= 1 {
						return log.FErrf("var array size must be greater than 1: %d", count)
					}
					resolver.AddVar(varName, cpu.ImmediateData(totalWords+count-1))
				} else {
					// Single variable
					resolver.AddVar(arg, cpu.ImmediateData(totalWords))
				}
				totalWords += count
			}
			op = op.SetOpcode(cpu.Push)
			op = op.SetOperand(cpu.ImmediateData(totalWords - 1))
			returnN = int(totalWords)
			log.Debugf("Var -> Push %d and defined variables: %v", totalWords-1, resolver.vars)
		case "param":
			// define more stack labels
			start := returnN + 1 // +1 to skip over the return PC
			for i := range narg {
				resolver.AddVar(args[i], cpu.ImmediateData(start+i))
			}
			log.Debugf("Param -> Defined parameters: %v", resolver.vars)
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
			//nolint:nestif // it's still readable.
			if instrEnum >= cpu.LoadS { // for stack instructions, resolve var references
				for i, v := range args {
					if !isAddressLabel(v) {
						continue
					}
					if intV, ok := resolver.Const(v); ok {
						log.Debugf("Resolved const %s to value %d", v, intV)
						args[i] = strconv.FormatInt(intV, 10)
						continue
					}
					if idx, ok := resolver.Var(v); ok {
						log.Debugf("Resolved var %s to index %d", v, idx)
						args[i] = strconv.FormatInt((int64)(idx), 10)
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
				remainingBits = 48 // syscall ID takes 8 bits
			case cpu.LoadSB, cpu.StoreSB:
				// Load/Store byte at stack index (first argument) with byte offset from stack index (second argument)
				instrName := "LoadSB"
				if instrEnum == cpu.StoreSB {
					instrName = "StoreSB"
				}
				v1, err := resolver.ResValue(args[0])
				if err != nil {
					return log.FErrf("Failed to parse argument %q: %v", args[0], err)
				}
				if v1 < 0 || v1 >= cpu.StackSize {
					return log.FErrf("%s stack base out of range (0 to %d): %d", instrName, cpu.StackSize-1, v1)
				}
				v2, err := resolver.ResValue(args[1])
				if err != nil {
					return log.FErrf("Failed to parse stack index argument %q: %v", args[1], err)
				}
				if v2 < 0 || v2 >= cpu.StackSize {
					return log.FErrf("%s byte offset stack index out of range (0 to %d): %d", instrName, cpu.StackSize-1, v2)
				}
				op = op.SetOperand(cpu.ImmediateData(v2))
				remainingBits = 48 // byte offset stack index takes 8 bits
				op = op.SetOperandWithBits(cpu.ImmediateData(v1), remainingBits)
			case cpu.IncrS:
				// Increment by delta (first argument) at stack index (second argument)
				v1, err := resolver.ResValue(args[0])
				if err != nil {
					return log.FErrf("Failed to parse argument %q: %v", args[0], err)
				}
				if v1 < -128 || v1 > 127 {
					return log.FErrf("IncrS immediate value out of range (-128 to 127): %d", v1)
				}
				v2, err := resolver.ResValue(args[1])
				if err != nil {
					return log.FErrf("Failed to parse stack index argument %q: %v", args[1], err)
				}
				if v2 < 0 || v2 >= cpu.StackSize {
					return log.FErrf("IncrS stack index out of range (0 to %d): %d", cpu.StackSize-1, v2)
				}
				op = op.SetOperand(cpu.ImmediateData(v1))
				remainingBits = 48 // increment delta takes 8 bits
				op = op.SetOperandWithBits(cpu.ImmediateData(v2), remainingBits)
			case cpu.IncrR:
				// 2 arguments: value (-128 to 127) and label
				label = args[1]
				v, err := resolver.ResValue(args[0])
				if err != nil {
					return log.FErrf("Failed to parse argument %q: %v", args[0], err)
				}
				if v < -128 || v > 127 {
					return log.FErrf("IncrR immediate value out of range (-128 to 127): %d", v)
				}
				op = op.SetOperand(cpu.ImmediateData(v))
				remainingBits = 48 // increment value takes 8 bits
			case cpu.JNE, cpu.JEQ, cpu.JLT, cpu.JGT, cpu.JGTE, cpu.JLTE:
				// 2 arguments: value to compare and label for destination
				label = args[1]
				v, err := resolver.ResValue(args[0])
				if err != nil {
					return log.FErrf("Failed to parse argument %q: %v", args[0], err)
				}
				if v < -128 || v > 127 {
					return log.FErrf("Jump comparison value out of range (-128 to 127): %d", v)
				}
				// Encode as: lower 8 bits = value, upper bits = destination (to be filled in by emitCode)
				op = op.SetOperand(cpu.ImmediateData(v))
				remainingBits = 48 // comparison value takes 8 bits
			default:
				// allow labels as arguments even for immediate operands (e.g. load the address into accumulator)
				v, err := resolver.ResValue(args[0])
				if err != nil {
					if isAddressLabel(arg) {
						label = arg
						break
					}
					return log.FErrf("Failed to parse argument %q: %v", arg, err)
				}
				op = op.SetOperand(cpu.ImmediateData(v))
			}
		}
		result = append(result, Line{Op: op, Label: label, Data: data, remainingBits: remainingBits})
		pc++
	}
	return emitCode(writer, result, resolver)
}

func emitCode(writer io.Writer, result []Line, resolver *Resolver) int {
	for pc, line := range result {
		op := line.Op
		if !line.Data && line.Label != "" {
			// resolve label
			targetPC, ok := resolver.Labels(line.Label)
			if !ok {
				return log.FErrf("Unknown label: %s for %#v", line.Label, line)
			}
			relativePC := targetPC - cpu.ImmediateData(pc)
			// Use SetOperandWithBits for all bit widths
			op = op.SetOperandWithBits(relativePC, line.remainingBits)
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
