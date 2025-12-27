package asm

import (
	"reflect"
	"testing"
)

func TestParseLine(t *testing.T) {
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
		{"data 'H'", []string{"data", "H"}},
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
		{`data '\''`, []string{"data", "'"}},
		// \ not special inside backticks
		{"data `a backslash: \\`", []string{"data", "a backslash: \\"}},
		// 2 word instruction example
		{"Sys Sleep\t250 # Comment", []string{"Sys", "Sleep", "250"}},
		// Escaped backslash
		// {`data "a\\"`, []string{"data", `a\\`}},
	}
	for _, line := range lines {
		t.Run(line.input, func(t *testing.T) {
			result, err := parseLine(line.input)
			if err != nil {
				t.Fatalf("parseLine(%q) returned error: %v", line.input, err)
			}
			if !reflect.DeepEqual(result, line.expected) {
				t.Errorf("parseLine(%q) = %v; want %v", line.input, result, line.expected)
			}
		})
	}
}

func TestParseLineErrors(t *testing.T) {
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
			result, err := parseLine(input)
			if err == nil {
				t.Errorf("parseLine(%q) = %v; expected error", input, result)
			}
		})
	}
}
