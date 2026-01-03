; cat.asm: like the Unix cat command, reads input from stdin and writes it to stdout
; test for instance with 1234567_10_234567_20_234567_30_234567_40

read:
    LoadI 32 ; read up to 32 bytes at a time
    Sys Read buf
    JGT 0 write ; proceed to write if any bytes were read
    JLT 0 error ; jump if error
    ; Last case 0 read == normal EOF case, no error:
    Sys Exit 0
write:
    Sys Write buf
    JGT 0 read
    ; write error
error:
    Sys Exit 1

; need (32+1) bytes for str8 so 5 words.
buf:
    data 0
    data 0
    data 0
    data 0
    data 0
