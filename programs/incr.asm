    LoadI 10
    storeR v
loop:
    incrR -1 v
    loadR v
    jnz loop
    incrR -42 v
    loadR v
end:
    sys exit 0 # actual exit
v:
    data 0
