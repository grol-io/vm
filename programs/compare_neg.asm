; countdown and prints from +3 to -3
;compile with
; vm compile programs/compare_neg.asm programs/itoa.asm
    var counter
    loadi 3
    StoreS counter
loop:
    call itoa
    incrs -1 counter
    jgte -3 loop
    sys exit 0
