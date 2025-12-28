package cpu

import (
	"bytes"
	"testing"
)

func TestOperandBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		operand ImmediateData
		valid   bool
	}{
		{
			name:    "minimum valid value",
			operand: -(1 << 55),
			valid:   true,
		},
		{
			name:    "maximum valid value",
			operand: (1 << 55) - 1,
			valid:   true,
		},
		{
			name:    "zero",
			operand: 0,
			valid:   true,
		},
		{
			name:    "positive middle value",
			operand: 1 << 54,
			valid:   true,
		},
		{
			name:    "negative middle value",
			operand: -(1 << 54),
			valid:   true,
		},
		{
			name:    "small positive value",
			operand: 42,
			valid:   true,
		},
		{
			name:    "small negative value",
			operand: -42,
			valid:   true,
		},
		{
			name:    "one below minimum (should panic)",
			operand: -(1 << 55) - 1,
			valid:   false,
		},
		{
			name:    "one above maximum (should panic)",
			operand: (1 << 55),
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var op Operation
			if tt.valid {
				// Should not panic
				op = op.SetOperand(tt.operand)
				// Test roundtrip
				got := op.Operand()
				if got != tt.operand {
					t.Errorf("roundtrip failed: SetOperand(%d).Operand() = %d, want %d", tt.operand, got, tt.operand)
				}
			} else {
				// Should panic
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("SetOperand(%d) did not panic, but should have", tt.operand)
					}
				}()
				op.SetOperand(tt.operand)
			}
		})
	}
}

func TestOperandRoundtrip(t *testing.T) {
	// Test a range of values for roundtrip correctness
	testValues := []ImmediateData{
		-(1 << 55),
		-(1 << 54),
		-(1 << 32),
		-(1 << 16),
		-1000,
		-1,
		0,
		1,
		1000,
		1 << 16,
		1 << 32,
		1 << 54,
		(1 << 55) - 1,
	}

	for _, val := range testValues {
		t.Run("", func(t *testing.T) {
			var op Operation
			op = op.SetOperand(val)
			got := op.Operand()
			if got != val {
				t.Errorf("roundtrip failed for %d: got %d", val, got)
			}
		})
	}
}

func TestOperandWithOpcode(t *testing.T) {
	// Test that setting operand doesn't affect opcode and vice versa
	var op Operation
	op = op.SetOpcode(AddI)
	op = op.SetOperand(42)

	if op.Opcode() != AddI {
		t.Errorf("Opcode() = %v, want %v", op.Opcode(), AddI)
	}
	if op.Operand() != 42 {
		t.Errorf("Operand() = %d, want 42", op.Operand())
	}

	// Set a different opcode, operand should remain
	op = op.SetOpcode(JNZ)
	if op.Opcode() != JNZ {
		t.Errorf("Opcode() = %v, want %v", op.Opcode(), JNZ)
	}
	if op.Operand() != 42 {
		t.Errorf("Operand() = %d, want 42", op.Operand())
	}

	// Set a different operand, opcode should remain
	op = op.SetOperand(-100)
	if op.Opcode() != JNZ {
		t.Errorf("Opcode() = %v, want %v", op.Opcode(), JNZ)
	}
	if op.Operand() != -100 {
		t.Errorf("Operand() = %d, want -100", op.Operand())
	}
}

func TestOperandRange(t *testing.T) {
	// Verify the documented range: -2^55 to 2^55-1
	minValue := -(1 << 55)
	maxValue := (1 << 55) - 1

	t.Logf("Operand range: %d to %d", minValue, maxValue)
	t.Logf("That's -%d to %d", 1<<55, (1<<55)-1)

	// Min should work
	var op Operation
	op = op.SetOperand(ImmediateData(minValue))
	if op.Operand() != ImmediateData(minValue) {
		t.Errorf("minimum value roundtrip failed: got %d, want %d", op.Operand(), minValue)
	}

	// Max should work
	op = op.SetOperand(ImmediateData(maxValue))
	if op.Operand() != ImmediateData(maxValue) {
		t.Errorf("maximum value roundtrip failed: got %d, want %d", op.Operand(), maxValue)
	}

	// One below min should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("value below minimum did not panic")
		}
	}()
	op.SetOperand(ImmediateData(minValue - 1))
}

func Test48BitsOperandBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		operand ImmediateData
		valid   bool
	}{
		{
			name:    "minimum valid value",
			operand: -(1 << 47),
			valid:   true,
		},
		{
			name:    "maximum valid value",
			operand: (1 << 47) - 1,
			valid:   true,
		},
		{
			name:    "zero",
			operand: 0,
			valid:   true,
		},
		{
			name:    "positive middle value",
			operand: 1 << 46,
			valid:   true,
		},
		{
			name:    "negative middle value",
			operand: -(1 << 46),
			valid:   true,
		},
		{
			name:    "small positive value",
			operand: 42,
			valid:   true,
		},
		{
			name:    "small negative value",
			operand: -42,
			valid:   true,
		},
		{
			name:    "one below minimum (should panic)",
			operand: -(1 << 47) - 1,
			valid:   false,
		},
		{
			name:    "one above maximum (should panic)",
			operand: (1 << 47),
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var op Operation
			if tt.valid {
				// Should not panic
				op = op.Set48BitsOperand(tt.operand)
				// Test roundtrip - extract 48-bit operand from bit 16 onwards
				got := ImmediateData(int64(op) >> 16)
				if got != tt.operand {
					t.Errorf("roundtrip failed: Set48BitsOperand(%d) -> extracted %d, want %d", tt.operand, got, tt.operand)
				}
			} else {
				// Should panic
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("Set48BitsOperand(%d) did not panic, but should have", tt.operand)
					}
				}()
				op.Set48BitsOperand(tt.operand)
			}
		})
	}
}

func Test48BitsOperandRoundtrip(t *testing.T) {
	// Test a range of values for roundtrip correctness
	testValues := []ImmediateData{
		-(1 << 47),
		-(1 << 46),
		-(1 << 32),
		-(1 << 16),
		-1000,
		-1,
		0,
		1,
		1000,
		1 << 16,
		1 << 32,
		1 << 46,
		(1 << 47) - 1,
	}

	for _, val := range testValues {
		t.Run("", func(t *testing.T) {
			var op Operation
			op = op.Set48BitsOperand(val)
			// Extract the 48-bit operand from bit 16 onwards
			got := ImmediateData(int64(op) >> 16)
			if got != val {
				t.Errorf("roundtrip failed for %d: got %d", val, got)
			}
		})
	}
}

func Test48BitsOperandWithOpcode(t *testing.T) {
	// Test that 48-bit operand preserves the lower 16 bits (2-byte opcode)
	var op Operation
	// Set a 2-byte opcode value in lower 16 bits
	op = Operation(0x1234)
	op = op.Set48BitsOperand(42)

	// Lower 16 bits should be preserved
	lowerBits := uint16(op & 0xFFFF)
	if lowerBits != 0x1234 {
		t.Errorf("Lower 16 bits = 0x%x, want 0x1234", lowerBits)
	}

	// Extract operand from bit 16 onwards
	operand := ImmediateData(int64(op) >> 16)
	if operand != 42 {
		t.Errorf("Operand = %d, want 42", operand)
	}

	// Set a different operand, lower 16 bits should remain
	op = op.Set48BitsOperand(-100)
	lowerBits = uint16(op & 0xFFFF)
	if lowerBits != 0x1234 {
		t.Errorf("Lower 16 bits = 0x%x, want 0x1234", lowerBits)
	}
	operand = ImmediateData(int64(op) >> 16)
	if operand != -100 {
		t.Errorf("Operand = %d, want -100", operand)
	}
}

func Test48BitsOperandRange(t *testing.T) {
	// Verify the documented range: -2^47 to 2^47-1
	minValue := -(1 << 47)
	maxValue := (1 << 47) - 1

	t.Logf("48-bit operand range: %d to %d", minValue, maxValue)
	t.Logf("That's -%d to %d", 1<<47, (1<<47)-1)

	// Min should work
	var op Operation
	op = op.Set48BitsOperand(ImmediateData(minValue))
	got := ImmediateData(int64(op) >> 16)
	if got != ImmediateData(minValue) {
		t.Errorf("minimum value roundtrip failed: got %d, want %d", got, minValue)
	}

	// Max should work
	op = op.Set48BitsOperand(ImmediateData(maxValue))
	got = ImmediateData(int64(op) >> 16)
	if got != ImmediateData(maxValue) {
		t.Errorf("maximum value roundtrip failed: got %d, want %d", got, maxValue)
	}

	// One below min should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("value below minimum did not panic")
		}
	}()
	op.Set48BitsOperand(ImmediateData(minValue - 1))
}

func TestSysPrint(t *testing.T) {
	tests := []struct {
		name     string
		memory   []Operation
		addr     ImmediateData
		expected string
		wantN    int64
	}{
		{
			name: "single byte string",
			memory: []Operation{
				// str8 format: byte 0 = length, bytes 1-7 = data
				// 'A' = 0x41
				// Byte 0: length = 1, Byte 1: 'A' = 0x41
				// Little-endian: 0x01 | (0x41 << 8) = 0x4101
				Operation(0x4101),
			},
			addr:     0,
			expected: "A",
			wantN:    1,
		},
		{
			name: "empty string",
			memory: []Operation{
				Operation(0x00), // length=0
			},
			addr:     0,
			expected: "",
			wantN:    0,
		},
		{
			name: "three byte string",
			memory: []Operation{
				// "Hi!" = H(0x48), i(0x69), !(0x21)
				// Byte 0: length = 3, Byte 1: 'H', Byte 2: 'i', Byte 3: '!'
				// Little-endian: 0x03 | (0x48 << 8) | (0x69 << 16) | (0x21 << 24)
				Operation(0x21694803),
			},
			addr:     0,
			expected: "Hi!",
			wantN:    3,
		},
		{
			name: "7 byte string (fits in first word)",
			memory: []Operation{
				// "Hello!!" = H(0x48), e(0x65), first-l(0x6C), second-l(0x6C), o(0x6F), !(0x21), !(0x21)
				// Byte 0: length=7, Bytes 1-7: Hello!!
				// 0x07 | (0x48<<8) | (0x65<<16) | (0x6C<<24) | (0x6C<<32) | (0x6F<<40) | (0x21<<48) | (0x21<<56)
				Operation(0x21216F6C6C654807),
			},
			addr:     0,
			expected: "Hello!!",
			wantN:    7,
		},
		{
			name: "8 byte string (requires second word)",
			memory: []Operation{
				// "Hello Wo" = H(0x48), e(0x65), first-l(0x6C), second-l(0x6C), o(0x6F), space(0x20), W(0x57), o(0x6F)
				// First word: length=8, bytes 1-7 = "Hello W"
				// 0x08 | (0x48<<8) | (0x65<<16) | (0x6C<<24) | (0x6C<<32) | (0x6F<<40) | (0x20<<48) | (0x57<<56)
				Operation(0x57206F6C6C654808),
				// Second word: byte 0 = 'o' (0x6F)
				Operation(0x6F),
			},
			addr:     0,
			expected: "Hello Wo",
			wantN:    8,
		},
		{
			name: "15 byte string (spans 2 continuation words)",
			memory: []Operation{
				// "Hello World, th"
				// First word: length=15 (0x0F), bytes 1-7 = "Hello W"
				Operation(0x57206F6C6C65480F),
				// Second word: bytes 0-7 = "orld, th"
				// o(0x6F), r(0x72), l(0x6C), d(0x64), comma(0x2C), space(0x20), t(0x74), h(0x68)
				// 0x6F | (0x72<<8) | (0x6C<<16) | (0x64<<24) | (0x2C<<32) | (0x20<<40) | (0x74<<48) | (0x68<<56)
				Operation(0x6874202C646C726F),
			},
			addr:     0,
			expected: "Hello World, th",
			wantN:    15,
		},
		{
			name: "string at offset",
			memory: []Operation{
				Operation(0x00), // dummy
				Operation(0x00), // dummy
				// "AB" = A(0x41), B(0x42)
				// Byte 0: length=2, Byte 1: 'A', Byte 2: 'B'
				// 0x02 | (0x41 << 8) | (0x42 << 16)
				Operation(0x424102),
			},
			addr:     2,
			expected: "AB",
			wantN:    2,
		},
		{
			name: "multi-byte unicode (test raw bytes)",
			memory: []Operation{
				// 3 bytes for UTF-8 encoding of U+2000 (EN QUAD): 0xE2 0x80 0x80
				// Byte 0: length=3, Byte 1: 0xE2, Byte 2: 0x80, Byte 3: 0x80
				// 0x03 | (0xE2 << 8) | (0x80 << 16) | (0x80 << 24)
				Operation(0x8080E203),
			},
			addr:     0,
			expected: "\u2000", // Unicode EN QUAD
			wantN:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			n := sysPrint(&buf, tt.memory, tt.addr)
			if n != tt.wantN {
				t.Errorf("sysPrint() returned %d, want %d", n, tt.wantN)
			}
			got := buf.String()
			if got != tt.expected {
				t.Errorf("sysPrint() output = %q, want %q", got, tt.expected)
			}
		})
	}
}
