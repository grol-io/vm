; loop.asm: Native version of the 1 billion iteration loop test
; Tests JNE conditional jump instruction
    # 1 billion iterations
    LoadI 1_000_000_000
loop:
    addI -1
    jne 0 loop
    sys exit 0 # actual exit
