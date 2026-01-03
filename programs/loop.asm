    # 1 billion iterations
    LoadI 1_000_000_000
loop:
    addI -1
    jne 0 loop
    loadI 1
    jne 0 end
    # this should be skipped
    loadI 23
    sys exit -7 # not ran
end:
    loadI -42
    sys exit 0 # actual exit
