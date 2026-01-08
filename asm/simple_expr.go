package asm

import (
	"strings"

	"fortio.org/log"
)

type Operator string

const (
	Plus        Operator = "+"
	Minus       Operator = "-"
	Mult        Operator = "*"
	Div         Operator = "/"
	Mod         Operator = "%"
	OperatorOr  Operator = "|"
	OperatorAnd Operator = "&"
	LeftShift   Operator = "<<"
	RightShift  Operator = ">>"
)

var allOperators = []Operator{ // with precedence
	Plus,
	Minus,
	Mult,
	Div,
	Mod,
	OperatorOr,
	OperatorAnd,
	LeftShift,
	RightShift,
}

// OperatorSplit splits between leftside and right side when finding one of our operators.
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

func (r *Resolver) ResOperator(op Operator, left, right string) (int64, error) {
	var leftV int64
	if left != "" {
		// to handle "-x" "+x" (will also yield 0 on *x etc but whatever)
		var err error
		leftV, err = r.ResValue(left)
		if err != nil {
			return 0, err
		}
	}
	rightV, err := r.ResValue(right)
	if err != nil {
		return 0, err
	}
	var res int64
	switch op {
	case Plus:
		res = leftV + rightV
	case Minus:
		res = leftV - rightV
	case Mult:
		res = leftV * rightV
	case Div:
		res = leftV / rightV
	case Mod:
		res = leftV % rightV
	case LeftShift:
		res = leftV << rightV
	case RightShift:
		res = leftV >> rightV
	case OperatorOr:
		res = leftV | rightV
	case OperatorAnd:
		res = leftV & rightV
	default:
		panic("unsupported operator: " + string(op))
	}
	log.LogVf("Evaluated expression: %s %s %s = %d", left, op, right, res)
	return res, nil
}
