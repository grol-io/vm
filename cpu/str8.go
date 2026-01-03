package cpu

import "fmt"

// Serialize serializes numbytes (<= 8) bytes of data into 1 int64.
func Serialize(b []byte) Operation {
	if len(b) == 0 || len(b) > 8 {
		panic("unsupported number of bytes")
	}
	var result uint64
	for i := len(b) - 1; i >= 0; i-- {
		result <<= 8
		result |= uint64(b[i])
	}
	return Operation(result) //nolint:gosec // no overflow, just bits shoving unsigned to signed.
}

// SerializeStr8 serializes a string of up to 255 bytes into a slice of Operations.
// TODO: consider using unsafe to put the data directly into memory without copying
// (like the cvm does).
func SerializeStr8(b []byte) []Operation {
	l := len(b)
	if l == 0 || l > 256 {
		panic(fmt.Sprintf("str8 can only handle strings 1-256 bytes, got %d", l))
	}
	var result []Operation
	// First word: up to 7 bytes of data + length byte
	firstChunkSize := min(l, 7)
	result = append(result, Serialize(b[:firstChunkSize])<<8|Operation(l-1))
	// Remaining bytes in chunks of 8
	remaining := b[firstChunkSize:]
	for len(remaining) > 0 {
		chunkSize := min(len(remaining), 8)
		result = append(result, Serialize(remaining[:chunkSize]))
		remaining = remaining[chunkSize:]
	}
	return result
}
