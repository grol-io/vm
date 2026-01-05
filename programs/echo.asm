; echo.asm: similar to the Unix echo command, prints back each argument but one per line

param sargc sargv
    LoadS sargc
    StoreR argc
    POP 1
loop:
    IncrR -1 argc
    JLT 0 end
    Pop 0
    Sys Write8 0 # if address is 0, address will be from A (which we just popped)
    Sys Write8 newline
    JumpR loop
end:
    Sys Exit 0
argc:
    data 0
newline:
    str8 "\n"
