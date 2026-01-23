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
	header   ELF64Header
	phdr     ELF64Phdr         // Single loadable segment for now
	code     []byte            // Machine code
	data     []byte            // Data section (embedded after code)
	dataAddr uint64            // Virtual address where data starts
	labels   map[string]uint64 // Label -> virtual address
	// For patching RIP-relative addresses
	patches []patch
}

type patch struct {
	codeOffset int // Offset in code where the 32-bit displacement is
	dataOffset int // Offset in data section that we're referencing
	instrEnd   int // Offset of end of instruction (RIP value when executed)
}

// NewELF64Binary creates a new ELF64 binary builder.
func NewELF64Binary() *ELF64Binary {
	e := &ELF64Binary{
		labels: make(map[string]uint64),
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
	e.header.Phnum = 1 // One loadable segment
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

// addPatch records a location that needs patching after code generation.
func (e *ELF64Binary) addPatch(codeOffset, dataOffset, instrEnd int) {
	e.patches = append(e.patches, patch{
		codeOffset: codeOffset,
		dataOffset: dataOffset,
		instrEnd:   instrEnd,
	})
}

// applyPatches fixes up RIP-relative addresses now that we know final layout.
func (e *ELF64Binary) applyPatches() {
	headerSize := Elf64HeaderSize + Elf64PhdrSize
	codeStart := headerSize
	dataStart := codeStart + len(e.code)

	for _, p := range e.patches {
		// Target address (in file offset terms, which equals virtual offset from base)
		targetFileOffset := dataStart + p.dataOffset
		// RIP value when instruction executes (file offset of instruction end)
		ripFileOffset := codeStart + p.instrEnd
		// Displacement = target - rip (fits in int32 for reasonable code sizes)
		disp := int32(targetFileOffset - ripFileOffset) //nolint:gosec // displacement fits in 32 bits for our use case
		// Patch the displacement in code
		binary.LittleEndian.PutUint32(e.code[p.codeOffset:], uint32(disp)) //nolint:gosec // int32 to uint32 is safe
	}
}

// Finalize prepares the binary for writing.
func (e *ELF64Binary) Finalize() {
	// Apply RIP-relative patches before calculating final sizes
	e.applyPatches()

	// Calculate total size: header + phdr + code + data
	headerSize := uint64(Elf64HeaderSize + Elf64PhdrSize)
	codeSize := uint64(len(e.code))
	dataSize := uint64(len(e.data))
	totalSize := headerSize + codeSize + dataSize

	// Entry point is right after headers
	entryPoint := Elf64BaseAddress + headerSize
	e.header.Entry = entryPoint

	// Set up the program header for a single loadable segment
	e.phdr.Type = PtLoad
	e.phdr.Flags = PfR | PfX // Read + Execute (could add PfW if needed)
	e.phdr.Offset = 0        // Start of file
	e.phdr.Vaddr = Elf64BaseAddress
	e.phdr.Paddr = Elf64BaseAddress
	e.phdr.Filesz = totalSize
	e.phdr.Memsz = totalSize
	e.phdr.Align = Elf64PageSize

	// Data starts after code
	e.dataAddr = entryPoint + codeSize
}

// WriteTo writes the complete ELF64 binary to the writer.
func (e *ELF64Binary) WriteTo(w io.Writer) error {
	// Write ELF header
	if err := binary.Write(w, binary.LittleEndian, &e.header); err != nil {
		return err
	}

	// Write program header
	if err := binary.Write(w, binary.LittleEndian, &e.phdr); err != nil {
		return err
	}

	// Write code
	if _, err := w.Write(e.code); err != nil {
		return err
	}

	// Write data
	if _, err := w.Write(e.data); err != nil {
		return err
	}

	return nil
}

// EmitELF64 generates a native Linux ELF64 binary from the parsed assembly.
func EmitELF64(writer io.Writer, result []Line, resolver *Resolver) int {
	elf := NewELF64Binary()

	// First pass: calculate code positions and resolve labels
	// For now, we'll do a simple translation

	// Track where data will be placed (after all code)
	var dataSection []byte
	dataOffsets := make(map[int]int) // VM PC -> offset in dataSection

	// Collect data items first
	for pc, line := range result {
		if line.Data {
			dataOffsets[pc] = len(dataSection)
			// Serialize the Operation as 8 bytes
			var buf [8]byte
			binary.LittleEndian.PutUint64(buf[:], uint64(line.Op)) //nolint:gosec // Operation is int64, uint64 cast is safe
			dataSection = append(dataSection, buf[:]...)
		}
	}

	// Generate code for each instruction
	for pc, line := range result {
		if line.Data {
			continue // Skip data, it goes at the end
		}

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
		case cpu.LoadI:
			// mov rax, immediate
			elf.emitMovImm(RAX, operand)

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
				if dataOffset, ok := dataOffsets[int(targetPC)]; ok {
					// Read length from str8 data (first byte)
					strLen := int(dataSection[dataOffset])

					// mov rdi, fd
					elf.emitMovImm32(RDI, int32(fd)) //nolint:gosec // fd is small

					// lea rsi, [rip + offset_to_data + 1] (skip length byte)
					dispOffset := elf.emitLeaRipRelative(RSI, 0) // Placeholder
					instrEnd := len(elf.code)
					elf.addPatch(dispOffset, dataOffset+1, instrEnd) // +1 to skip length byte

					// mov rdx, length
					elf.emitMovImm32(RDX, int32(strLen)) //nolint:gosec // str8 length <= 255

					// mov rax, sysWrite (1)
					elf.emitMovImm32(RAX, sysWrite)
					// syscall
					elf.emitSyscall()
				} else {
					log.Warnf("Write8 target PC %d (from PC %d + offset %d) not found in data offsets", targetPC, pc, addr)
				}

			default:
				log.Warnf("Unimplemented syscall: %v", syscallID)
			}

		default:
			log.Warnf("Unimplemented instruction: %v", opcode)
		}
	}

	// Add data section
	elf.data = dataSection

	// Finalize and write
	elf.Finalize()

	if err := elf.WriteTo(writer); err != nil {
		return log.FErrf("Failed to write ELF64: %v", err)
	}

	return 0
}
