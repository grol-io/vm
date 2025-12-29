package asm

import (
	"bufio"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	lines := []struct {
		input    string
		expected []string
	}{
		{"LoadI 42", []string{"LoadI", "42"}},
		{"AddI -1", []string{"AddI", "-1"}},
		{"JNZ loop", []string{"JNZ", "loop"}},
		// comments:
		{"# This is a comment", nil},
		{"   # This is a comment with leading spaces", nil},
		// comments at the end of a line
		{"LoadI 42 # This is a comment at the end of a line", []string{"LoadI", "42"}},
		// String literals:
		{`data "Hello, World!"`, []string{"data", "Hello, World!"}},
		// with special characters
		{`data "a \t\n\r\\b"`, []string{"data", "a \t\n\r\\b"}},
		// with # in the string
		{`data "a # b"`, []string{"data", "a # b"}},
		// Unicode characters
		{`data "こんにちは"`, []string{"data", "こんにちは"}},
		// Single quotes
		{"data 'H'", []string{"data", "0x48"}},
		// Backticks
		{`data ` + "`Hello, World!\\n`", []string{"data", "Hello, World!\\n"}},
		// other quotes inside a quoted string
		{
			`data "He said, 'Hello, World!'` + " and a backtick ` inside\"",
			[]string{"data", "He said, 'Hello, World!' and a backtick ` inside"},
		},
		{"data `He said, \"Hello, World!\"`", []string{"data", "He said, \"Hello, World!\""}},
		// \" doesn't terminate the string
		{`data "He said, \"Hello, World!\""`, []string{"data", "He said, \"Hello, World!\""}},
		// \' doesn't terminate the character
		{`data '\''`, []string{"data", "0x27"}},
		// \ not special inside backticks
		{"data `a backslash: \\`", []string{"data", "a backslash: \\"}},
		// 2 word instruction example
		{"Sys Sleep\t250 # Comment", []string{"Sys", "Sleep", "250"}},
		// Escaped backslash
		{`data "a\\"`, []string{"data", `a\`}},
		{`data '\\'`, []string{"data", "0x5c"}},
		{"data `\\`", []string{"data", `\`}},
		{"data `\\\\`", []string{"data", `\\`}},
	}
	for _, line := range lines {
		t.Run(line.input, func(t *testing.T) {
			for i := range 2 {
				inp := line.input
				if i == 1 {
					inp += "\nanother line\n"
				}
				reader := bufio.NewReader(strings.NewReader(inp))
				result, err := parse(reader)
				if err != nil {
					t.Fatalf("parse(%q) returned error: %v", inp, err)
				}
				if !reflect.DeepEqual(result, line.expected) {
					t.Errorf("parse(%q) = %v; want %v", inp, result, line.expected)
				}
			}
		})
	}
}

func TestParseMultiline(t *testing.T) {
	// Test multi-line backtick string
	multiLineInput := "# a comment first\n\tdata `hello\nworld\ntest`"
	reader := bufio.NewReader(strings.NewReader(multiLineInput))
	result, err := parse(reader)
	if err != nil {
		t.Fatalf("parse(%q) returned error: %v", multiLineInput, err)
	}
	// first the comment -> empty result
	if len(result) != 0 {
		t.Errorf("parse(%q) = %v; want empty result due to comment", multiLineInput, result)
	}
	// now parse the next line data line
	result, err = parse(reader)
	if err != nil {
		t.Fatalf("parse(%q) returned error: %v", multiLineInput, err)
	}
	expected := []string{"data", "hello\nworld\ntest"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("parse(%q) = %v; want %v", multiLineInput, result, expected)
	}
	// now check we get eof
	result, err = parse(reader)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("parse(%q) returned error: %v instead of EOF", multiLineInput, err)
	}
	if len(result) != 0 {
		t.Errorf("parse(%q) = %v; want empty result due to EOF", multiLineInput, result)
	}
}

func TestParseErrors(t *testing.T) {
	errorCases := []string{
		`abc"d ef"`,       // quote in middle of token
		`data abc"hello"`, // quote in middle of token
		`foo"bar" baz`,    // quote in middle of token
		`data "a b`,       // unterminated quote
		`"hello world`,    // unterminated quote at start
		`x "y`,            // unterminated quote
		`data "\x"`,       // invalid hex escape
		`data "\u123"`,    // incomplete unicode escape
		`data "\"`,        // backslash at end
		`data "\xZZ"`,     // invalid hex digits
		`data 'AB'`,       // more than 1 rune in single quotes
		`data "ab'`,       // unterminated quote/wrong quote
	}
	for _, input := range errorCases {
		t.Run(input, func(t *testing.T) {
			for i := range 2 {
				inp := input
				if i == 1 {
					inp += "\nanother line\n"
				}
				reader := bufio.NewReader(strings.NewReader(inp))
				result, err := parse(reader)
				if err == nil {
					t.Errorf("parse(%q) = %v; expected error", inp, result)
				}
			}
		})
	}
}

func TestSerializeStr8(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedLines int
		checkFirst    bool
		firstOp       uint64 // expected first operation value
	}{
		{
			name:          "single byte",
			input:         "A",
			expectedLines: 1,
			checkFirst:    true,
			firstOp:       0x4101, // length 1 in byte 0, 'A' (0x41) in byte 1
		},
		{
			name:          "two bytes",
			input:         "AB",
			expectedLines: 1,
			checkFirst:    true,
			firstOp:       0x424102, // 'A' (0x41), 'B' (0x42), length 2
		},
		{
			name:          "seven bytes fits in one line",
			input:         "ABCDEFG",
			expectedLines: 1,
			checkFirst:    true,
			firstOp:       0x47464544434241_07, // 7 chars + length byte
		},
		{
			name:          "eight bytes needs two words",
			input:         "ABCDEFGH",
			expectedLines: 2,
		},
		{
			name:          "fifteen bytes needs two words",
			input:         "ABCDEFGHIJKLMNO",
			expectedLines: 2, // 7 in first line + 8 in second
		},
		{
			name:          "16 bytes needs three words (7 + 8 + 1)",
			input:         "0123456789ABCDEF",
			expectedLines: 3,
		},
		{
			name:          "255 bytes max",
			input:         string(make([]byte, 255)),
			expectedLines: 32, // 1 + (255-7)/8 = 1 + 31 = 32
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeStr8([]byte(tt.input))
			if len(result) != tt.expectedLines {
				t.Errorf("serializeStr8(%q) returned %d lines, expected %d", tt.input, len(result), tt.expectedLines)
			}
			// All lines should have Data flag set
			for i, line := range result {
				if !line.Data {
					t.Errorf("Line %d should have Data=true", i)
				}
				if line.Label != "" {
					t.Errorf("Line %d should have empty Label", i)
				}
				if line.Is48bit {
					t.Errorf("Line %d should have Is48bit=false", i)
				}
			}
			// Check first line encoding
			if tt.checkFirst && len(result) > 0 {
				firstOp := uint64(result[0].Op)
				if firstOp != tt.firstOp {
					t.Errorf("First operation = 0x%x, expected 0x%x", firstOp, tt.firstOp)
				}
				// Verify length byte
				lengthByte := firstOp & 0xFF
				if lengthByte != uint64(len(tt.input)) {
					t.Errorf("Length byte = %d, expected %d", lengthByte, len(tt.input))
				}
			}
		})
	}
}

func TestSerializeStr8Panics(t *testing.T) {
	panicTests := []struct {
		name  string
		input []byte
	}{
		{"empty string", []byte{}},
		{"256 bytes", make([]byte, 256)},
		{"1000 bytes", make([]byte, 1000)},
	}

	for _, tt := range panicTests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("serializeStr8 should panic for %s", tt.name)
				}
			}()
			serializeStr8(tt.input)
		})
	}
}
