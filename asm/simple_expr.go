package asm

import (
	"strings"

	"fortio.org/log"
)

type Operator string

const (
	Plus       Operator = "+"
	Minus      Operator = "-"
	Mult       Operator = "*"
	Div        Operator = "/"
	Mod        Operator = "%"
	BitOr      Operator = "|"
	BitAnd     Operator = "&"
	LeftShift  Operator = "<<"
	RightShift Operator = ">>"
)

var allOperators = []Operator{ // with precedence
	Plus,
	Minus,
	Mult,
	Div,
	Mod,
	BitOr,
	BitAnd,
	LeftShift,
	RightShift,
}

// OperatorSplit splits between leftside and right side when finding one of our operators.
// While not very efficient (but then it also doesn't matter too much for what we are compiling)
// it also handles precedence (unlike what would happen if we were to use IndexAny for the
// many 1 byte operators instance).
func OperatorSplit(str string) (Operator, string, string) {
	for _, op := range allOperators {
		if idx := strings.LastIndex(str, string(op)); idx != -1 {
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
	case BitOr:
		res = leftV | rightV
	case BitAnd:
		res = leftV & rightV
	default:
		panic("unsupported operator: " + string(op))
	}
	log.LogVf("Evaluated expression: %s %s %s = %d", left, op, right, res)
	return res, nil
}
