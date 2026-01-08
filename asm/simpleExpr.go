package asm

import "strings"

type Operator string

const (
	Plus       Operator = "+"
	Minus      Operator = "-"
	Mult       Operator = "*"
	Div        Operator = "/"
	Mod        Operator = "%"
	LeftShift  Operator = "<<"
	RightShift Operator = ">>"
)

var allOperators = []Operator{
	Plus,
	Minus,
	Mult,
	Div,
	Mod,
	LeftShift,
	RightShift,
}

// TODO: Not very efficient (but then it also doesn't matter too much for what we are compiling):
// Replace by IndexAny for the 1 char first 5 operators and check the shifts after that ?
func OperatorSplit(str string) (Operator, string, string) {
	for _, op := range allOperators {
		if idx := strings.Index(str, string(op)); idx != -1 {
			return op, str[:idx], str[idx+len(op):]
		}
	}
	return "", "", ""
}
