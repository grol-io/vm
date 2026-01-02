; cat.asm: like the Unix cat command, reads input from stdin and writes it to stdout
; test for instance with 1234567_10_234567_20_234567_30_234567_40
    var _ _ _ _ buf ; reserve enough space for 32 bytes (first byte is size so we need 1 more word)
read:
    LoadI 32 ; read up to 32 bytes at a time
    SysS Read buf
    SubI 1
    JPOS write
    JNEG error
    ; normal EOF case, no error:
    SysS Exit 0
write:
    LoadI 0 ; no byte offset within buf, str8 from the start.
    SysS Write buf
    JPOS read
    ; write error
error:
    SysS Exit 1
