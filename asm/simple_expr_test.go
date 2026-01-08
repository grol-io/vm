package asm

import (
	"testing"

	"fortio.org/log"
)

func TestSimplePrecedence(t *testing.T) {
	log.SetLogLevelQuiet(log.Debug)
	tsts := []struct {
		input  string
		result int64
	}{
		{"2+3*5", 17},
		{"3*5+2", 17},
		{"0x20|0x03", 0x23},
		{"0x3<<4|0x5", 0x35},
		{"0x5|0x3<<4", 0x35},
		{"-23", -23},
		{"5-7", -2},
		{"42%5", 2},
		{"42/5", 8},
		{"0xff&0x2345", 0x45},
		{"0x2345>>4", 0x0234},
		{"10-3-2", 5},
		{"1+2|3", 4}, // treat as 1+(2|3) on purpose
		{"2|3+1", 4},
	}
	r := NewResolver()
	for _, tst := range tsts {
		v, err := r.ResValue(tst.input)
		if err != nil {
			t.Errorf("unexpected error for input %q: %v", tst.input, err)
		}
		if v != tst.result {
			t.Errorf("unexpected result for input %q: got %d, want %d", tst.input, v, tst.result)
		}
	}
}
