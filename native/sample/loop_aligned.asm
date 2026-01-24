; loop_aligned.asm: Native loop test with alignment padding
; Loop target at 0x80 (16-byte aligned)
    LoadI 1_000_000_000
    nop  ; pad to align loop to 16-byte boundary
    nop  ; LoadI is 5 bytes at 0x78, so 0x7d. 3 NOPs -> 0x80
    nop
loop:
    addI -1
    jne 0 loop
    sys exit 0
