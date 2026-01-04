; wc.asm: like `wc -l`, counts newline characters in stdin and prints the count
; Compile with: vm compile programs/wc.asm programs/itoa.asm

    ; reserve 256 bytes for buffer (b1-b31+buf)=32 words=32*8=256 bytes
    Var count bytes_read b1 b2 b3 b4 b5 b6 b7 b8 b9 b10 b11 b12 b13 b14 b15 b16 b17 b18 b19 b20 b21 b22 b23 b24 b25 b26 b27 b28 b29 b30 b31 buf; buf extends into the b*
    LoadI 0
    StoreS count

read_loop:
    LoadI 256 ; read up to 256 bytes at a time
    SysS ReadN buf
    JLTE 0 check_result
    ; Process bytes (accumulator has bytes_read)
    StoreS bytes_read
count_loop:
    IncrS -1 bytes_read
    LoadSB buf bytes_read
    JNE '\n' not_newline
    IncrS 1 count ; found newline, count it
not_newline:
    LoadS bytes_read
    JGT 0 count_loop
    JumpR read_loop

check_result:
    JEQ 0 no_error
    Sys Exit 1
no_error:
    LoadS count
    Call itoa
    Sys Exit 0
