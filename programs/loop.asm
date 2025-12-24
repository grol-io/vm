    # 1 billion iterations
    load 1_000_000_000
loop:
    add -1
    jnz loop
    load 1
    jnz end
    # this should be skipped
    load 23
    exit -7
end:
    load -42
    exit 0
