# itoa: convert the number in `num` to decimal using loops, store into a str8 word, and print it.
# Builds digits least-significant-first with ModI/DivI 10, then prefixes length byte.
# Handles negative numbers including min_int64 (once we increase buf to more than 1 word/7chars).

    LoadI -1
    ShiftI 63 # -9223372036854775808 will be truncated to 7 characters for now.
    CALL itoa
    LoadI 7890
    CALL itoa
    LoadI -123456
    CALL itoa
    LoadI 1234567
    CALL itoa
    LoadI 0
    CALL itoa
    Sys exit 0

itoa: # prints accumulator as a decimal string
    Push 4 # reserve 4 additional entries on stack: num:0 + is_negative:1, digit:2, buf:3, len:4
    JPOS digits_loop
    # else mark/remember as negative to add the minus sign at the end.
    LoadI 1
    StoreS 1 # is_negative
    LoadS 0 # num

digits_loop:
    ModI 10
    JPOS positive_digit
      MulI -1 # We don't just do that to the initial number because of min_int64
  positive_digit:
    AddI '0'
    StoreS 2 # digit

    LoadS 3 # buf
    ShiftI 8
    AddS 2 # digit
    StoreS 3  # buf
    IncrS 1 4 # len by 1

    LoadS 0 # num
    DivI 10
    StoreS 0 # num
    JNZ digits_loop

done:
    LoadS 1 # is_negative
    JNZ add_minus
    LoadS 3 # buf
finish_str:
    ShiftI 8
    AddS 4 # len
    StoreS 3 # buf

    SysS write 3 # buf
    Call println
    Return 5 # 4 + accumulator

add_minus:
    IncrS 1 4 # len
    LoadS 3 # buf
    ShiftI 8
    AddI '-'
    JumpR finish_str

println:
    Sys write newline
    Return 0
newline:
    str8 "\n"
