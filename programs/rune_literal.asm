.const newline '\n'
.const ascii_r 'R'
.const STDOUT 1

    LoadI newline
    ShiftI 8
    AddI ascii_r
    ShiftI 8
    AddI 2 # len
    StoreR buf
    Sys Write8 STDOUT buf
    Sys exit 0
buf:
    data 0
