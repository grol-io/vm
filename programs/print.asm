# itoa: convert the number in `num` to decimal using loops, store into a str8 word, and print it.
# Builds digits least-significant-first with ModI/DivI 10, then prefixes length byte.

    LoadI 170890
    StoreR num

    LoadI 0
    StoreR buf
    StoreR len

    LoadR num
    JNZ digits_loop

# Special-case zero
    LoadI 0
    StoreR digit

    LoadR buf
    ShiftI 8
    StoreR buf

    LoadR digit
    AddI 48
    AddR buf
    StoreR buf

    LoadI 1
    StoreR len
    JNZ finalize

digits_loop:
    LoadR num
    ModI 10
    StoreR digit

    LoadR buf
    ShiftI 8
    StoreR buf

    LoadR digit
    AddI 48
    AddR buf
    StoreR buf

    LoadR len
    AddI 1
    StoreR len

    LoadR num
    DivI 10
    StoreR num

    LoadR num
    JNZ digits_loop

finalize:
    LoadR buf
    ShiftI 8
    StoreR buf

    LoadR buf
    AddR len
    StoreR buf

    Sys write buf
    Sys write newline
    Sys exit 0

num:
    data 0
buf:
    data 0
len:
    data 0
digit:
    data 0
newline:
    str8 "\n"
