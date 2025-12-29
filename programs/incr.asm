    LoadI 10
    StoreR v
loop:
    IncrR -1 v
    JNZ loop
# Try another bigger increment than 1/-1: should yield 0-42 == -42 in accumulator
    IncrR -42 v
    Sys Exit 0
v:
    data 0
