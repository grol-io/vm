//go:build windows

package cpu

import (
	"errors"
	"fmt"
	"io"
	"os"
	"unsafe"

	"fortio.org/log"
)

// signalSetup is a no-op on Windows (SIGPIPE doesn't exist).
func signalSetup() {
	// No-op on Windows
}

func sysRead(in int64, memory []Operation, addr, n int) int64 {
	if n < 0 {
		panic(fmt.Sprintf("invalid read size: %d", n))
	}
	if n == 0 {
		log.LogVf("Read size is 0, nothing to read")
		return 0
	}
	if len(memory) == 0 {
		panic("memory slice is empty")
	}
	// Cast the memory operations to a byte slice using unsafe
	// Each Operation is an int64, so we need addr*OperationSize bytes offset
	memAsBytes := unsafe.Slice((*byte)(unsafe.Pointer(&memory[0])), len(memory)*OperationSize)
	byteOffset := addr * OperationSize
	// Use os.NewFile for Windows Handle compatibility
	f := os.NewFile(uintptr(in), "")
	if f == nil {
		log.Errf("Invalid file descriptor: %d", in)
		return -1
	}
	// Note: Don't close f since we don't own the fd - it's passed in and may be reused
	r, err := f.Read(memAsBytes[byteOffset : byteOffset+n])
	if err != nil && !errors.Is(err, io.EOF) {
		log.Errf("Failed to read: %v", err)
		return -1
	}
	log.LogVf("Read %d bytes from fd %d", r, in)
	return int64(r)
}

func sysRead8(in int64, memory []Operation, addr, n int) int64 {
	if n <= 0 || n > 255 {
		panic(fmt.Sprintf("invalid read size for str8: %d", n))
	}
	if len(memory) == 0 {
		panic("memory slice is empty")
	}
	// Cast the memory operations to a byte slice using unsafe
	// Each Operation is an int64, so we need addr*OperationSize bytes offset
	memAsBytes := unsafe.Slice((*byte)(unsafe.Pointer(&memory[0])), len(memory)*OperationSize)

	// For str8, the length byte goes at byteOffset 0, data starts at byteOffset 1
	byteOffset := addr * OperationSize

	// Use os.NewFile for Windows Handle compatibility
	f := os.NewFile(uintptr(in), "")
	if f == nil {
		log.Errf("Invalid file descriptor: %d", in)
		return -1
	}
	// Note: Don't close f since we don't own the fd - it's passed in and may be reused
	r, err := f.Read(memAsBytes[byteOffset+1 : byteOffset+1+n])
	if err != nil && !errors.Is(err, io.EOF) {
		log.Errf("Failed to read8: %v", err)
		return -1
	}
	log.LogVf("Read8 %d bytes from fd %d", r, in)
	if r == 0 {
		return 0
	}
	// Set the length byte
	memAsBytes[byteOffset] = byte(r)
	return int64(r)
}

// sysWrite8 writes the str8 bytes and returns the number of bytes it did output.
func sysWrite8(out int64, memory []Operation, addr, offset int) int64 {
	log.LogVf("Writing fd %d str8 from memory at addr: %d, offset: %d", out, addr, offset)
	if len(memory) == 0 {
		panic("memory slice is empty")
	}
	// Cast the memory operations to a byte slice using unsafe
	// Each Operation is an int64, so we need addr*OperationSize bytes offset
	memAsBytes := unsafe.Slice((*byte)(unsafe.Pointer(&memory[0])), len(memory)*OperationSize)

	byteOffset := addr*OperationSize + offset
	length := int(memAsBytes[byteOffset])
	if length == 0 {
		return 0
	}
	if log.LogVerbose() {
		// this would alloc a slice so we avoid it unless verbose logging is enabled
		log.LogVf("Before writing bytes: %d %q", length, memAsBytes[byteOffset+1:byteOffset+1+length])
	}
	// Use os.NewFile for Windows Handle compatibility
	f := os.NewFile(uintptr(out), "")
	if f == nil {
		log.Errf("Invalid file descriptor: %d", out)
		return -1
	}
	// Note: Don't close f since we don't own the fd - it's passed in and may be reused
	// Write directly from memory without copying
	n, err := f.Write(memAsBytes[byteOffset+1 : byteOffset+1+length])
	log.LogVf("Wrote %d bytes to stdout (err %v)", n, err)

	if err != nil {
		log.Errf("Failed to output str8: %v", err)
		return -1
	}
	if n != length {
		log.Errf("Failed to output all bytes: expected %d, got %d", length, n)
		return -1
	}
	return int64(length)
}

// sysWrite writes the n bytes and returns the number of bytes it did output.
func sysWrite(out int64, memory []Operation, addr, n int) int64 {
	log.LogVf("Writing fd %d n bytes from memory at addr: %d, n: %d", out, addr, n)
	if n < 0 {
		panic(fmt.Sprintf("invalid write size: %d", n))
	}
	if n == 0 {
		return 0
	}
	if len(memory) == 0 {
		panic("memory slice is empty")
	}
	// Cast the memory operations to a byte slice using unsafe
	// Each Operation is an int64, so we need addr*OperationSize bytes offset
	memAsBytes := unsafe.Slice((*byte)(unsafe.Pointer(&memory[0])), len(memory)*OperationSize)
	byteOffset := addr * OperationSize
	if log.LogVerbose() {
		// this would alloc a slice so we avoid it unless verbose logging is enabled
		log.LogVf("Before writing bytes: %d %q", n, memAsBytes[byteOffset:byteOffset+n])
	}
	// Use os.NewFile for Windows Handle compatibility
	f := os.NewFile(uintptr(out), "")
	if f == nil {
		log.Errf("Invalid file descriptor: %d", out)
		return -1
	}
	// Note: Don't close f since we don't own the fd - it's passed in and may be reused
	// Write directly from memory without copying
	m, err := f.Write(memAsBytes[byteOffset : byteOffset+n])
	log.LogVf("Wrote %d bytes to stdout (err %v)", m, err)
	if err != nil {
		log.Errf("Failed to output bytes: %v", err)
		return -1
	}
	if n != m {
		log.Errf("Failed to output all bytes: expected %d, got %d", n, m)
		return -1
	}
	return int64(n)
}
