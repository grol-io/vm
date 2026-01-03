    LoadI '\n'
    ShiftI 8
    AddI 'R'
    ShiftI 8
    AddI 4 # len
    StoreR buf
    Sys Write8 buf
    Sys exit 0
buf:
    data 0
