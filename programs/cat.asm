; cat.asm: like the Unix cat command, reads input from stdin and writes it to stdout

; read up to 4096 bytes at a time; note this is close to the full stack size (512*8 bytes)
.const bufsize 4096
.const stackbuf -1 ; hack reads to the blank stack (stack_ptr -  -1 = next stack slot)

read:
    LoadI bufsize
    SysS ReadN stackbuf
    JGT 0 write ; proceed to write if any bytes were read
    JLT 0 error ; jump if error
    ; Last case 0 read == normal EOF case, no error:
    Sys Exit 0
write:
    SysS WriteN stackbuf ; same buffer as read
    JGT 0 read
    ; write error
error:
    Sys Exit 1
