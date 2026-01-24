package asm

import (
	"encoding/binary"
	"io"

	"fortio.org/log"
	"grol.io/vm/cpu"
)

// ELF64 constants.
const (
	ElfMag0          = 0x7f     // ELF magic byte 0
	ElfMag1          = 'E'      // ELF magic byte 1
	ElfMag2          = 'L'      // ELF magic byte 2
	ElfMag3          = 'F'      // ELF magic byte 3
	ElfClass64       = 2        // 64-bit
	ElfData2LSB      = 1        // Little-endian
	EvCurrent        = 1        // ELF version
	ElfOSABINone     = 0        // UNIX System V ABI
	EtExec           = 2        // Executable file
	EmX8664          = 62       // AMD x86-64
	PtLoad           = 1        // Loadable segment
	PfX              = 0x1      // Execute
	PfW              = 0x2      // Write
	PfR              = 0x4      // Read
	Elf64HeaderSize  = 64       // ELF64 header size in bytes
	Elf64PhdrSize    = 56       // Program header size in bytes
	Elf64PageSize    = 0x1000   // Page size for alignment
	Elf64BaseAddress = 0x400000 // Traditional Linux base address
)

// ELF64Header represents the ELF64 file header.
type ELF64Header struct {
	Ident     [16]byte // ELF identification
	Type      uint16   // Object file type
	Machine   uint16   // Machine type
	Version   uint32   // Object file version
	Entry     uint64   // Entry point address
	Phoff     uint64   // Program header offset
	Shoff     uint64   // Section header offset (0 = none)
	Flags     uint32   // Processor-specific flags
	Ehsize    uint16   // ELF header size
	Phentsize uint16   // Size of program header entry
	Phnum     uint16   // Number of program header entries
	Shentsize uint16   // Size of section header entry
	Shnum     uint16   // Number of section header entries
	Shstrndx  uint16   // Section name string table index
}

// ELF64Phdr represents a program header entry.
type ELF64Phdr struct {
	Type   uint32 // Segment type
	Flags  uint32 // Segment flags
	Offset uint64 // Offset in file
	Vaddr  uint64 // Virtual address in memory
	Paddr  uint64 // Physical address (usually same as Vaddr)
	Filesz uint64 // Size in file
	Memsz  uint64 // Size in memory
	Align  uint64 // Alignment
}

// ELF64Binary holds all the components needed to build an ELF64 executable.
type ELF64Binary struct {
	header      ELF64Header
	textPhdr    ELF64Phdr         // Code segment (R+X)
	dataPhdr    ELF64Phdr         // Data segment (R+W)
	code        []byte            // Machine code
	rodata      []byte            // Read-only data (constants, strings)
	data        []byte            // Writable data (buffers, variables)
	dataAddr    uint64            // Virtual address where writable data starts
	dataPadding int               // Padding bytes between text and data in file (for page alignment)
	labels      map[string]uint64 // Label -> virtual address
	// For patching RIP-relative addresses to data
	patches []patch
	// For patching jump displacements
	jumpPatches   []jumpPatch
	pcToCodeStart map[int]int // VM PC -> offset in code where that instruction starts
}

type patch struct {
	codeOffset int  // Offset in code where the 32-bit displacement is
	dataOffset int  // Offset in data section that we're referencing
	instrEnd   int  // Offset of end of instruction (RIP value when executed)
	isWritable bool // True if referencing writable data section, false for rodata
}

// NewELF64Binary creates a new ELF64 binary builder.
func NewELF64Binary() *ELF64Binary {
	e := &ELF64Binary{
		labels:        make(map[string]uint64),
		pcToCodeStart: make(map[int]int),
	}

	// Initialize ELF header
	e.header.Ident[0] = ElfMag0
	e.header.Ident[1] = ElfMag1
	e.header.Ident[2] = ElfMag2
	e.header.Ident[3] = ElfMag3
	e.header.Ident[4] = ElfClass64
	e.header.Ident[5] = ElfData2LSB
	e.header.Ident[6] = EvCurrent
	e.header.Ident[7] = ElfOSABINone
	// Ident[8..15] are padding (zeros)

	e.header.Type = EtExec
	e.header.Machine = EmX8664
	e.header.Version = EvCurrent
	e.header.Phoff = Elf64HeaderSize // Program headers immediately follow ELF header
	e.header.Shoff = 0               // No section headers
	e.header.Flags = 0
	e.header.Ehsize = Elf64HeaderSize
	e.header.Phentsize = Elf64PhdrSize
	e.header.Phnum = 2 // Two loadable segments: text (R+X) and data (R+W)
	e.header.Shentsize = 0
	e.header.Shnum = 0
	e.header.Shstrndx = 0

	return e
}

// x86-64 Linux syscall numbers.
const (
	sysWrite = 1
	sysExit  = 60
)

// x86-64 register encodings (for ModR/M and REX).
const (
	RAX = 0
	RCX = 1
	RDX = 2
	RBX = 3
	RSP = 4
	RBP = 5
	RSI = 6
	RDI = 7
	R8  = 8
	R9  = 9
	R10 = 10
	R11 = 11
	R12 = 12
	R13 = 13
	R14 = 14
	R15 = 15
)

// Emit helpers for x86-64 instructions.
func (e *ELF64Binary) emitBytes(bytes ...byte) {
	e.code = append(e.code, bytes...)
}

// emitMovImm32 emits: mov reg, imm32 (zero-extended to 64-bit).
// Use for values 0 to 2^31-1.
func (e *ELF64Binary) emitMovImm32(reg int, imm int32) {
	if reg >= 8 {
		e.emitBytes(0x41) // REX.B
		reg -= 8
	}
	e.emitBytes(byte(0xB8 + reg)) // MOV r32, imm32
	// Little-endian immediate
	for i := range 4 {
		e.emitBytes(byte(imm >> (i * 8)))
	}
}

// emitMovImm64 emits: mov reg, imm64 (REX.W + B8+rd + imm64).
// Use for values that don't fit in 32 bits.
func (e *ELF64Binary) emitMovImm64(reg int, imm int64) {
	if reg >= 8 {
		e.emitBytes(0x49) // REX.WB
		reg -= 8
	} else {
		e.emitBytes(0x48) // REX.W
	}
	e.emitBytes(byte(0xB8 + reg)) // MOV r64, imm64
	// Little-endian immediate
	for i := range 8 {
		e.emitBytes(byte(imm >> (i * 8)))
	}
}

// emitMovImm emits the most efficient mov instruction for the given immediate.
// - Values 0 to 2^31-1: mov r32, imm32 (5-6 bytes, zero-extends).
// - Other values: mov r64, imm64 (10 bytes).
func (e *ELF64Binary) emitMovImm(reg int, imm int64) {
	// If value fits in unsigned 32-bit, use shorter encoding (zero-extends)
	if imm >= 0 && imm <= 0x7FFFFFFF {
		e.emitMovImm32(reg, int32(imm))
	} else {
		e.emitMovImm64(reg, imm)
	}
}

// emitAddImm emits: add reg, imm (sign-extended 32-bit immediate).
func (e *ELF64Binary) emitAddImm(reg int, imm int64) {
	// ADD: RAX special opcode 0x05, ModR/M base 0xC0
	e.emitALUImm(reg, imm, 0x05, 0xC0)
}

// emitSubImm emits: sub reg, imm (sign-extended 32-bit immediate).
func (e *ELF64Binary) emitSubImm(reg int, imm int64) {
	// SUB: RAX special opcode 0x2D, ModR/M base 0xE8
	e.emitALUImm(reg, imm, 0x2D, 0xE8)
}

// emitALUImm emits an ALU operation with immediate: op reg, imm
// raxOpcode is the special opcode for RAX with 32-bit immediate (e.g., 0x05 for ADD, 0x2D for SUB)
// modRMBase is the base ModR/M byte (e.g., 0xC0 for ADD, 0xE8 for SUB).
func (e *ELF64Binary) emitALUImm(reg int, imm int64, raxOpcode, modRMBase byte) {
	// REX.W for 64-bit operation
	if reg >= 8 {
		e.emitBytes(0x49) // REX.WB
		reg -= 8
	} else {
		e.emitBytes(0x48) // REX.W
	}
	switch {
	case reg == RAX && (imm < -128 || imm > 127):
		// Special encoding for RAX with 32-bit immediate
		e.emitBytes(raxOpcode)
	case imm >= -128 && imm <= 127:
		// r/m64, imm8 (sign-extended)
		e.emitBytes(0x83, modRMBase+byte(reg))
	default:
		// r/m64, imm32 (sign-extended)
		e.emitBytes(0x81, modRMBase+byte(reg))
	}
	// Emit immediate
	if imm >= -128 && imm <= 127 {
		e.emitBytes(byte(imm))
	} else {
		for i := range 4 {
			e.emitBytes(byte(imm >> (i * 8)))
		}
	}
}

// emitSyscall emits the syscall instruction.
func (e *ELF64Binary) emitSyscall() {
	e.emitBytes(0x0F, 0x05)
}

// emitLeaRipRelative emits: lea reg, [rip + offset]
// offset is calculated from the end of this instruction
// Returns the offset in code where the 32-bit displacement is stored (for patching).
func (e *ELF64Binary) emitLeaRipRelative(reg int, offset int32) int {
	// REX prefix
	if reg >= 8 {
		e.emitBytes(0x4C) // REX.WR
		reg -= 8
	} else {
		e.emitBytes(0x48) // REX.W
	}
	e.emitBytes(0x8D)                    // LEA
	e.emitBytes(byte((reg << 3) | 0x05)) // ModR/M: reg, [rip+disp32]
	dispOffset := len(e.code)            // Remember where displacement goes
	// Little-endian displacement
	for i := range 4 {
		e.emitBytes(byte(offset >> (i * 8)))
	}
	return dispOffset
}

// emitMovFromRipRelative emits: mov reg, [rip + offset]
// Returns the offset in code where the 32-bit displacement is stored (for patching).
func (e *ELF64Binary) emitMovFromRipRelative(reg int) int {
	// REX.W prefix for 64-bit operand
	if reg >= 8 {
		e.emitBytes(0x4C) // REX.WR
		reg -= 8
	} else {
		e.emitBytes(0x48) // REX.W
	}
	e.emitBytes(0x8B)                    // MOV r64, r/m64
	e.emitBytes(byte((reg << 3) | 0x05)) // ModR/M: reg, [rip+disp32]
	dispOffset := len(e.code)            // Remember where displacement goes
	// Placeholder displacement (will be patched)
	e.emitBytes(0, 0, 0, 0)
	return dispOffset
}

// emitMovToRipRelative emits: mov [rip + offset], reg
// Returns the offset in code where the 32-bit displacement is stored (for patching).
func (e *ELF64Binary) emitMovToRipRelative(reg int) int {
	// REX.W prefix for 64-bit operand
	if reg >= 8 {
		e.emitBytes(0x4C) // REX.WR
		reg -= 8
	} else {
		e.emitBytes(0x48) // REX.W
	}
	e.emitBytes(0x89)                    // MOV r/m64, r64
	e.emitBytes(byte((reg << 3) | 0x05)) // ModR/M: [rip+disp32], reg
	dispOffset := len(e.code)            // Remember where displacement goes
	// Placeholder displacement (will be patched)
	e.emitBytes(0, 0, 0, 0)
	return dispOffset
}

// emitCmpImm emits: cmp reg, imm (sign-extended 8-bit or 32-bit immediate).
// For imm == 0, uses the more efficient "test reg, reg" instruction.
func (e *ELF64Binary) emitCmpImm(reg int, imm int64) {
	// Special case: comparing to 0 is more efficient with TEST reg, reg
	if imm == 0 {
		// TEST r64, r64 sets ZF=1 if reg is zero (same effect as CMP reg, 0)
		if reg >= 8 {
			e.emitBytes(0x4D) // REX.WRB (both source and dest are high registers)
			reg -= 8
		} else {
			e.emitBytes(0x48) // REX.W
		}
		// TEST r64, r64: opcode 0x85, ModR/M with mod=11, reg=reg, r/m=reg
		e.emitBytes(0x85, byte(0xC0|(reg<<3)|reg))
		return
	}
	// REX.W for 64-bit operation
	if reg >= 8 {
		e.emitBytes(0x49) // REX.WB
		reg -= 8
	} else {
		e.emitBytes(0x48) // REX.W
	}
	if imm >= -128 && imm <= 127 {
		// CMP r/m64, imm8 (sign-extended)
		e.emitBytes(0x83, byte(0xF8+reg))
		e.emitBytes(byte(imm))
	} else {
		// CMP r/m64, imm32 (sign-extended)
		// For RAX, there's a special encoding: 3D imm32
		if reg == RAX {
			e.emitBytes(0x3D)
		} else {
			e.emitBytes(0x81, byte(0xF8+reg))
		}
		for i := range 4 {
			e.emitBytes(byte(imm >> (i * 8)))
		}
	}
}

// Conditional jump opcodes (near, 32-bit displacement).
const (
	x86JNE = 0x85 // Jump if not equal (ZF=0)
	x86JE  = 0x84 // Jump if equal (ZF=1)
	x86JL  = 0x8C // Jump if less (SF≠OF)
	x86JGE = 0x8D // Jump if greater or equal (SF=OF)
	x86JG  = 0x8F // Jump if greater (ZF=0 and SF=OF)
	x86JLE = 0x8E // Jump if less or equal (ZF=1 or SF≠OF)
)

// emitCondJump emits a conditional near jump with 32-bit displacement.
// Returns the offset in code where the displacement is stored (for patching).
func (e *ELF64Binary) emitCondJump(condOpcode byte) int {
	e.emitBytes(0x0F, condOpcode) // Two-byte opcode for near conditional jump
	dispOffset := len(e.code)
	e.emitBytes(0, 0, 0, 0) // 32-bit displacement placeholder
	return dispOffset
}

// jumpPatch records a code location that needs a jump displacement patched.
type jumpPatch struct {
	codeOffset int // Offset in code where the 32-bit displacement is
	instrEnd   int // Offset of end of instruction (for RIP-relative calculation)
	targetPC   int // Target VM PC (assembly instruction index)
}

// addPatch records a location that needs patching after code generation.
func (e *ELF64Binary) addPatch(codeOffset, dataOffset, instrEnd int, isWritable bool) {
	e.patches = append(e.patches, patch{
		codeOffset: codeOffset,
		dataOffset: dataOffset,
		instrEnd:   instrEnd,
		isWritable: isWritable,
	})
}

// applyPatches fixes up RIP-relative addresses now that we know final layout.
// dataVaddr must be the virtual address where writable data starts.
func (e *ELF64Binary) applyPatches(numPhdrs int, dataVaddr uint64) {
	headerSize := Elf64HeaderSize + numPhdrs*Elf64PhdrSize
	codeStart := headerSize
	rodataStart := codeStart + len(e.code)

	for _, p := range e.patches {
		var targetVaddr uint64
		if p.isWritable {
			// Target is in writable data segment (page-aligned)
			targetVaddr = dataVaddr + uint64(p.dataOffset) //nolint:gosec // dataOffset is small
		} else {
			// Target is in rodata (text segment)
			targetVaddr = Elf64BaseAddress + uint64(rodataStart+p.dataOffset) //nolint:gosec // offsets are small
		}
		// RIP value when instruction executes
		ripVaddr := Elf64BaseAddress + uint64(codeStart+p.instrEnd) //nolint:gosec // offsets are small
		// Displacement = target - rip
		disp := int32(int64(targetVaddr) - int64(ripVaddr)) //nolint:gosec // displacement fits in 32 bits
		// Patch the displacement in code
		binary.LittleEndian.PutUint32(e.code[p.codeOffset:], uint32(disp)) //nolint:gosec // int32 to uint32 is safe
	}
}

// Finalize prepares the binary for writing.
func (e *ELF64Binary) Finalize() {
	// Determine number of program headers (skip data segment if empty)
	numPhdrs := 1 // Text segment always present
	hasDataSegment := len(e.data) > 0
	if hasDataSegment {
		numPhdrs = 2
	}
	e.header.Phnum = uint16(numPhdrs) //nolint:gosec // numPhdrs is 1 or 2

	// Calculate sizes
	headerSize := uint64(Elf64HeaderSize + numPhdrs*Elf64PhdrSize) //nolint:gosec // small values
	codeSize := uint64(len(e.code))
	rodataSize := uint64(len(e.rodata))
	textSize := headerSize + codeSize + rodataSize // Text segment includes headers, code, and rodata
	dataSize := uint64(len(e.data))

	// Calculate data segment virtual address BEFORE patching
	var dataVaddr uint64
	if hasDataSegment {
		// Data must be on a DIFFERENT page than text (conflicting permissions)
		dataVaddr = (Elf64BaseAddress + textSize + Elf64PageSize - 1) &^ (Elf64PageSize - 1)
	}

	// Apply RIP-relative patches now that we know final layout
	e.applyPatches(numPhdrs, dataVaddr)

	// Entry point is right after headers
	entryPoint := Elf64BaseAddress + headerSize
	e.header.Entry = entryPoint

	// Text segment: headers + code + rodata (Read + Execute)
	e.textPhdr.Type = PtLoad
	e.textPhdr.Flags = PfR | PfX
	e.textPhdr.Offset = 0
	e.textPhdr.Vaddr = Elf64BaseAddress
	e.textPhdr.Paddr = Elf64BaseAddress
	e.textPhdr.Filesz = textSize
	e.textPhdr.Memsz = textSize
	e.textPhdr.Align = Elf64PageSize

	if hasDataSegment {
		// Data segment: writable data (Read + Write)
		// IMPORTANT: Data must be on a DIFFERENT page than text because they have
		// conflicting permissions (R+X vs R+W). If they share a page, the second
		// mmap overwrites the first's permissions, causing SIGSEGV.
		//
		// File offset must be congruent to vaddr modulo page size.
		// Since dataVaddr is page-aligned, offset must also be page-aligned.
		dataOffset := (textSize + Elf64PageSize - 1) &^ (Elf64PageSize - 1)
		e.dataPadding = int(dataOffset - textSize) //nolint:gosec // padding is always < page size

		e.dataPhdr.Type = PtLoad
		e.dataPhdr.Flags = PfR | PfW
		e.dataPhdr.Offset = dataOffset
		e.dataPhdr.Vaddr = dataVaddr
		e.dataPhdr.Paddr = dataVaddr
		e.dataPhdr.Filesz = dataSize
		e.dataPhdr.Memsz = dataSize
		e.dataPhdr.Align = Elf64PageSize

		// Store data virtual address for later use
		e.dataAddr = dataVaddr
	}
}

// WriteTo writes the complete ELF64 binary to the writer.
func (e *ELF64Binary) WriteTo(w io.Writer) error {
	// Write ELF header
	if err := binary.Write(w, binary.LittleEndian, &e.header); err != nil {
		return err
	}

	// Write text program header
	if err := binary.Write(w, binary.LittleEndian, &e.textPhdr); err != nil {
		return err
	}

	// Write data program header only if we have writable data
	if len(e.data) > 0 {
		if err := binary.Write(w, binary.LittleEndian, &e.dataPhdr); err != nil {
			return err
		}
	}

	// Write code
	if _, err := w.Write(e.code); err != nil {
		return err
	}

	// Write read-only data
	if _, err := w.Write(e.rodata); err != nil {
		return err
	}

	// Write padding to align data segment to page boundary
	if e.dataPadding > 0 {
		padding := make([]byte, e.dataPadding)
		if _, err := w.Write(padding); err != nil {
			return err
		}
	}

	// Write writable data
	if _, err := w.Write(e.data); err != nil {
		return err
	}

	return nil
}

// EmitELF64 generates a native Linux ELF64 binary from the parsed assembly.
//
//nolint:gocognit,funlen,maintidx // code generator has inherent complexity
func EmitELF64(writer io.Writer, result []Line, resolver *Resolver) int {
	elf := NewELF64Binary()

	// Separate read-only data (str8) from writable data (data, .space)
	// Track where each data item will be placed
	var rodataSection []byte
	var dataSection []byte
	rodataOffsets := make(map[int]int) // VM PC -> offset in rodataSection
	dataOffsets := make(map[int]int)   // VM PC -> offset in dataSection
	isWritable := make(map[int]bool)   // VM PC -> true if writable

	// Collect data items first, separating by type
	for pc, line := range result {
		if !line.Data {
			continue
		}
		// Serialize the Operation as 8 bytes
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(line.Op)) //nolint:gosec // Operation is int64, uint64 cast is safe

		if line.ReadOnly {
			// str8 strings go to rodata (read-only)
			rodataOffsets[pc] = len(rodataSection)
			rodataSection = append(rodataSection, buf[:]...)
			isWritable[pc] = false
		} else {
			// data and .space go to writable data segment
			dataOffsets[pc] = len(dataSection)
			dataSection = append(dataSection, buf[:]...)
			isWritable[pc] = true
		}
	}

	// Generate code for each instruction
	for pc, line := range result {
		if line.Data {
			continue // Skip data, it goes at the end
		}

		// Track where this VM PC's code starts (for jump patching)
		elf.pcToCodeStart[pc] = len(elf.code)

		op := line.Op
		opcode := op.Opcode()
		operand := op.OperandInt64()

		// Resolve label if needed
		if line.Label != "" {
			targetPC, ok := resolver.Labels(line.Label)
			if !ok {
				return log.FErrf("Unknown label: %s", line.Label)
			}
			relativePC := int64(targetPC) - int64(pc)
			operand = relativePC
			// Repack with remaining bits
			if line.remainingBits < 56 {
				// Extract existing packed data
				existing := op.OperandInt64() & ((1 << (56 - line.remainingBits)) - 1)
				operand = (relativePC << (56 - line.remainingBits)) | existing
			}
		}

		switch opcode {
		case cpu.Nop:
			// x86-64 NOP (0x90)
			elf.emitBytes(0x90)

		case cpu.LoadI:
			// mov rax, immediate
			elf.emitMovImm(RAX, operand)

		case cpu.AddI:
			// add rax, immediate
			elf.emitAddImm(RAX, operand)
		case cpu.SubI:
			// sub rax, immediate (reuse emitAddImm with negated value)
			elf.emitSubImm(RAX, operand)
		case cpu.JNE:
			// Jump if not equal: compare RAX with value, jump if not equal
			// Operand layout: [48-bit jump offset][8-bit compare value]
			compareVal := int64(int8(operand & 0xFF)) //nolint:gosec // we want sign extension
			jumpOffset := operand >> 8
			targetPC := pc + int(jumpOffset)

			// cmp rax, compareVal
			elf.emitCmpImm(RAX, compareVal)
			// jne <displacement> (placeholder, will be patched)
			dispOffset := elf.emitCondJump(x86JNE)
			instrEnd := len(elf.code)

			// Record for later patching
			elf.jumpPatches = append(elf.jumpPatches, jumpPatch{
				codeOffset: dispOffset,
				instrEnd:   instrEnd,
				targetPC:   targetPC,
			})

		case cpu.LoadR:
			// Load from PC-relative address: mov rax, [rip + offset]
			targetPC := int64(pc) + operand
			if writable, ok := isWritable[int(targetPC)]; ok {
				var dataOffset int
				if writable {
					dataOffset = dataOffsets[int(targetPC)]
				} else {
					dataOffset = rodataOffsets[int(targetPC)]
				}
				dispOffset := elf.emitMovFromRipRelative(RAX)
				instrEnd := len(elf.code)
				elf.addPatch(dispOffset, dataOffset, instrEnd, writable)
			} else {
				return log.FErrf("LoadR target PC %d not found in data", targetPC)
			}

		case cpu.StoreR:
			// Store to PC-relative address: mov [rip + offset], rax
			targetPC := int64(pc) + operand
			if writable, ok := isWritable[int(targetPC)]; ok {
				if !writable {
					return log.FErrf("StoreR to read-only data at PC %d", targetPC)
				}
				dataOffset := dataOffsets[int(targetPC)]
				dispOffset := elf.emitMovToRipRelative(RAX)
				instrEnd := len(elf.code)
				elf.addPatch(dispOffset, dataOffset, instrEnd, true)
			} else {
				return log.FErrf("StoreR target PC %d not found in data", targetPC)
			}

		case cpu.Sys:
			// Extract syscall ID and argument (syscall ID is in low 8 bits, safe to convert)
			syscallID := cpu.Syscall(operand & 0xFF) //nolint:gosec // masked to 8 bits
			syscallArg := operand >> 8

			switch syscallID {
			case cpu.Exit:
				// mov rdi, exit_code
				elf.emitMovImm32(RDI, int32(syscallArg)) //nolint:gosec // exit codes are small
				// mov rax, sysExit (60)
				elf.emitMovImm32(RAX, sysExit)
				// syscall
				elf.emitSyscall()

			case cpu.Write8:
				// For now, handle the simple case where address is in the operand
				addr := syscallArg >> 8
				fd := syscallArg & 0xFF

				targetPC := int64(pc) + addr
				// Check both rodata and data sections
				var dataOffset int
				var writable bool
				var strData []byte

				if offset, ok := rodataOffsets[int(targetPC)]; ok {
					dataOffset = offset
					writable = false
					strData = rodataSection
				} else if offset, ok := dataOffsets[int(targetPC)]; ok {
					dataOffset = offset
					writable = true
					strData = dataSection
				} else {
					return log.FErrf("Write8 target PC %d (from PC %d + offset %d) not found in data offsets", targetPC, pc, addr)
				}

				// Read length from str8 data (first byte)
				strLen := int(strData[dataOffset])

				// mov rdi, fd
				elf.emitMovImm32(RDI, int32(fd)) //nolint:gosec // fd is small

				// lea rsi, [rip + offset_to_data + 1] (skip length byte)
				dispOffset := elf.emitLeaRipRelative(RSI, 0) // Placeholder
				instrEnd := len(elf.code)
				elf.addPatch(dispOffset, dataOffset+1, instrEnd, writable) // +1 to skip length byte

				// mov rdx, length
				elf.emitMovImm32(RDX, int32(strLen)) //nolint:gosec // str8 length <= 255

				// mov rax, sysWrite (1)
				elf.emitMovImm32(RAX, sysWrite)
				// syscall
				elf.emitSyscall()

			default:
				return log.FErrf("Unimplemented syscall: %v", syscallID)
			}

		default:
			return log.FErrf("Unimplemented instruction: %v", opcode)
		}
	}

	// Add data sections
	elf.rodata = rodataSection
	elf.data = dataSection

	// Apply jump patches now that we know all code offsets
	for _, jp := range elf.jumpPatches {
		targetCodeStart, ok := elf.pcToCodeStart[jp.targetPC]
		if !ok {
			return log.FErrf("Jump target PC %d not found in code", jp.targetPC)
		}
		// Displacement is relative to the end of the jump instruction
		disp := int32(targetCodeStart - jp.instrEnd)                          //nolint:gosec // displacement fits in 32 bits
		binary.LittleEndian.PutUint32(elf.code[jp.codeOffset:], uint32(disp)) //nolint:gosec // int32 to uint32 is safe
	}

	// Finalize and write
	elf.Finalize()

	if err := elf.WriteTo(writer); err != nil {
		return log.FErrf("Failed to write ELF64: %v", err)
	}

	return 0
}
