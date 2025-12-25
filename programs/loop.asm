    # 1 billion iterations
    LoadI 1_000_000_000
loop:
    addI -1
    jnz loop
    loadI 1
    jnz end
    # this should be skipped
    loadI 23
    exitI -7
end:
    loadI -42
    exitI 0
