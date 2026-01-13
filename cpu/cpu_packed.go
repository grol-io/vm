package cpu

import (
	"fortio.org/log"
)

func executePackedSyscall(syscall Syscall, operand, accumulator int64,
	memory []Operation, pc ImmediateData, isStack bool,
) (int64, bool) {
	// Packed arguments: FD (8 bits), Len (8 bits), Addr (Rest)
	fd := int(operand & 0xFF)
	length := int((operand >> 8) & 0xFF)
	addrParam := operand >> 16

	var targetAddr int64
	if addrParam == 0 {
		targetAddr = accumulator
	} else {
		targetAddr = int64(pc) + addrParam
	}

	if isStack {
		log.Errf("%v syscall not fully supported in Stack mode yet", syscall)
		return -1, false
	}

	switch syscall {
	case Read:
		return sysRead(fd, memory, int(targetAddr), length), false
	case Write:
		return sysWrite(fd, memory, int(targetAddr), length), false
	case Open:
		// Open uses targetAddr as the address of the filename (str8).
		// FD and Length arguments are ignored/unused by convention (should be 0).
		return sysOpen(memory, int(targetAddr)), false
	default:
		// Should not happen as we only call this for Read/Write
		log.Errf("executePackedSyscall called with unsupported syscall: %v", syscall)
		return -1, false
	}
}
