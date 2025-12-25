package cpu

import (
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
	op = op.SetOpcode(Load)
	if op.Opcode() != Load {
		t.Errorf("Opcode() = %v, want %v", op.Opcode(), Load)
	}
	if op.Operand() != 42 {
		t.Errorf("Operand() = %d, want 42", op.Operand())
	}

	// Set a different operand, opcode should remain
	op = op.SetOperand(-100)
	if op.Opcode() != Load {
		t.Errorf("Opcode() = %v, want %v", op.Opcode(), Load)
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
