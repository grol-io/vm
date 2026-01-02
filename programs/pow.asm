; Demo of a subroutine that takes more than 1 input argument
; (note that one of the arguments would probably be fine in accumulator but passing both
; via the stack is more demonstrative)
    loadi 5
var base exp # accumulator (so: 2) pushed to stack slot 0, creates exp slot 1
    loadi 3
    stores exp
    call pow
    sys exit 0 ; note we leave the 2 variables in stack on exit which is fine for this demo

pow:
    ; inside the pow subroutine, s0 is the return address so s1 is the first argument (base) and s2 is the second argument (exp)
    loadi 1  ; initialize result to 1
  var result ; now we pushed one more so exp is at slot 3 and base at slot 2
  param b e  ; b = base, e = exp, labels for their positions in the stack before PC
  loop:
    loads result
    muls b
    stores result
    incrs -1 e
    jnz loop
    loads result
    return
