; echo.asm: similar to the Unix echo command, prints back each argument but one per line
    POP 0 ; retrieve argc
    JEQ 0 end ; no args
    StoreR argc ; store it in memory (can't use the stack as we need to pop argvs 1 by 1)
loop:
    Pop 0
    Sys Write8 0 ; if address is 0, address will be from A (which we just popped)
    Sys Write8 newline
    IncrR -1 argc
    JNE 0 loop
end:
    Sys Exit 0
argc:
    data 0
newline:
    str8 "\n"
