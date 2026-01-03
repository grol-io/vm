; cat.asm: like the Unix cat command, reads input from stdin and writes it to stdout
; test for instance with 1234567_10_234567_20_234567_30_234567_40

read:
    LoadI 4096 ; read up to 4096 bytes at a time; note this match the full stack size (512*8 bytes)
    SysS ReadN -1 ; hack reads to the blank stack (stack_ptr -  -1 = next stack slot)
    JGT 0 write ; proceed to write if any bytes were read
    JLT 0 error ; jump if error
    ; Last case 0 read == normal EOF case, no error:
    Sys Exit 0
write:
    SysS WriteN -1 ; same buffer as read
    JGT 0 read
    ; write error
error:
    Sys Exit 1
