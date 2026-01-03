; factorial
; depends on itoa, so compile with
; vm compile programs/fact.asm programs/itoa.asm

    sys write fact_rec_str
    loadI 5
    call print
    call factrec
    call itoa
    sys write fact_iter_str
    loadI 7
    call print
    call facti
    call itoa
    sys exit 0

factrec: ; recursive factorial
    var n
    jgte 2 more
    loadI 1
    return
more:
    subi 1
    call factrec
    muls n
    return

facti: ; iterative factorial
    var n result
    loadI 1
    stores result
  loop:
    loadS n
    jlte 1 end
    muls result
    storeS result
    incrs -1 n
    jumpr loop
end:
    loadS result
    return

; print accumulator and put its value back (instead of the bytes written returned by itoa)
print:
    var acc
    sys write fact_str
    loadS acc
    call itoa
    loadS acc
    return

fact_rec_str:
    str8 "Recursive "
fact_iter_str:
    str8 "Iterative "
fact_str:
    str8 "Factorial of "
