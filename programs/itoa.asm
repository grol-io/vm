# itoa: convert the number in `num` to decimal using loops, store into a str8 word, and print it.
# Builds digits least-significant-first with ModI/DivI 10, then prefixes length byte.
# Handles negative numbers including min_int64 (once we increase buf to more than 1 word/7chars).

    # LoadI -1
    # ShiftI 63
    # LoadI 0
    # LoadI 12345
    LoadI -12345

itoa: # prints accumulator as a decimal string
    StoreR num
    # clear temp storage
    LoadI 0
    StoreR buf
    StoreR len
    StoreR is_negative

    LoadR num
    JPOS digits_loop
    LoadI 1
    StoreR is_negative
    LoadR num

digits_loop:
    ModI 10
    JPOS positive_digit
      MulI -1 # We don't just do that to the initial number because of min_int64
  positive_digit:
    AddI '0'
    StoreR digit

    LoadR buf
    ShiftI 8
    AddR digit
    StoreR buf
    IncrR 1 len

    LoadR num
    DivI 10
    StoreR num
    JNZ digits_loop

done:
    LoadR is_negative
    JNZ add_minus
    LoadR buf
finish_str:
    ShiftI 8
    AddR len
    StoreR buf

    Sys write buf
    Sys write newline
    Sys exit 0

add_minus:
    IncrR 1 len
    LoadR buf
    ShiftI 8
    AddI '-'
    JumpR finish_str

is_negative:
    data 0
num:
    data 0
digit:
    data 0
buf:
    data 0
    data 0
    data 0
len:
    data 0
newline:
    str8 "\n"
