package cpu

import "strings"

type Syscall uint8

const (
	InvalidSyscall Syscall = iota // skip 0 / avoid / detects accidental 0s

	Exit   // Exit with A as return code
	Read8  // Read a str8 string from stdin for up to A len bytes, result stored in param address/stack.
	Write8 // Print (output) a str8 string to stdout - pointed at by param (and for SysS A as byte offset from said stack entry)
	ReadN  // Read A bytes to address in param
	WriteN // Write A bytes from address in param (so very different use of A than SysS Write8)
	Sleep  // Sleep for A milliseconds

	LastSyscall
)

//go:generate stringer -type=Syscall
var _ = LastSyscall.String() // force compile error if go generate is missing.

var str2syscall map[string]Syscall

func init() {
	str2syscall = make(map[string]Syscall, LastSyscall)
	for i := InvalidSyscall + 1; i < LastSyscall; i++ {
		str2syscall[strings.ToLower(i.String())] = i
	}
}

// SyscallFromString converts a string (which must be lowercase) to a Syscall.
func SyscallFromString(s string) (Syscall, bool) {
	syscall, ok := str2syscall[s]
	return syscall, ok
}
