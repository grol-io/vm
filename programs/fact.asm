; factorial
; depends on iotoa, so compile with
; vm compile programs/fact.asm programs/itoa.asm

    sys write fact_rec_str
    loadI 5
    call itoa
    loadI 5
    call factrec
    call itoa
    sys write fact_iter_str
    loadI 7
    call itoa
    loadI 7
    call facti
    call itoa
    sys exit 0

factrec: ; recursive factorial
    var n
    subi 1
    jnz more
    loadI 1
    return
more:
    call factrec
    muls n
    return

facti: ; iterative factorial
    var result n
    subi 1
    storeS n
  loop:
    muls result
    storeS result
    incrs -1 n
    jnz loop
end:
    loadS result
    return


fact_rec_str:
    str8 "Recursive Factorial of "
fact_iter_str:
    str8 "Iterative Factorial of "
