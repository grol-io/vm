package cpu

import "strings"

type Syscall uint8

const (
	InvalidSyscall Syscall = iota // skip 0 / avoid / detects accidental 0s
	Exit
	Sleep
	Write
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
