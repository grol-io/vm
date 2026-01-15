; wc.asm: like `wc -l`, counts newline characters in stdin and prints the count
; Compile with: vm compile programs/wc.asm programs/itoa.asm


.const STDIN 0
; reserve and read 256 bytes at a time
.const maxBytes 256
; so for buffer; 32 words * 8 bytes = 256 bytes
.const maxWords maxBytes/8

    Var count bytes_read buf[maxWords]
    LoadI 0
    StoreS count

read_loop:
    LoadI maxBytes
    SysS ReadN STDIN buf
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
