# itoa: convert the number in `num` to decimal using loops, store into a str8 word, and print it.
# Builds digits least-significant-first with ModI/DivI 10, then prefixes length byte.
# Handles negative numbers by checking sign, negating, and adding '-' at the end.

    LoadI -708901
    StoreR num

    LoadI 0
    StoreR buf
    StoreR len
    StoreR negative

    LoadR num
    ShiftI -63
    AndI 1
    StoreR negative
    JNZ negate
    JNZ digits_loop
    LoadI 1
    JNZ zero_case

negate:
    LoadI 0
    SubR num
    StoreR num
    JNZ digits_loop

zero_case:
# Special-case zero
    LoadI 0
    StoreR digit

    LoadR buf
    ShiftI 8
    StoreR buf

    LoadR digit
    AddR zero
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
    AddR zero
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
    LoadR negative
    JNZ add_minus

finish_str:
    LoadR buf
    ShiftI 8
    StoreR buf

    LoadR buf
    AddR len
    StoreR buf

    Sys write buf
    Sys write newline
    Sys exit 0

add_minus:
    LoadR buf
    ShiftI 8
    StoreR buf

    LoadR minus_sign
    AddR buf
    StoreR buf

    LoadR len
    AddI 1
    StoreR len

    LoadI 1
    JNZ finish_str

minus_sign:
    data 45
zero:
    data 0x30
negative:
    data 0
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
