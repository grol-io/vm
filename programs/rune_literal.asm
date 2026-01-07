.const newline '\n'
.const ascii_r 'R'

    LoadI newline
    ShiftI 8
    AddI ascii_r
    ShiftI 8
    AddI 2 # len
    StoreR buf
    Sys Write8 buf
    Sys exit 0
buf:
    data 0
