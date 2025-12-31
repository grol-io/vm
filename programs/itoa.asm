# itoa: convert the number in `num` to decimal using loops, store into a str8 word, and print it.
# Builds digits least-significant-first with ModI/DivI 10, then prefixes length byte.
# Handles negative numbers including min_int64 (once we increase buf to more than 1 word/7chars).

# Simple case to debug:
#    LoadI -47
#    CALL itoa
#    Sys Exit 0 # for now to shorten the current debug
# Rest/normal tests:

    LoadI -1
    ShiftI 63 # -9223372036854775808
    CALL itoa

    LoadI 7890
    CALL itoa
    LoadI -123456
    CALL itoa
    LoadI 1234567
    CALL itoa
    LoadI 1234567890
    CALL itoa
    LoadI 0
    CALL itoa
    Sys exit 0

itoa: # prints accumulator as a decimal string
    Var num sign len _ _ buf # -> Push 5 reserve 5 additional entries on stack
    LoadI 21
    StoreS len
    # Add the newline
    LoadI '\n'
    StoreSB buf len # stores newline in buf(5) at offset indicated by len(2)
    IncrS -1 len
    LoadI 1
    StoreS sign
    LoadS num
    JPOS digits_loop
    # else mark/remember as negative to add the minus sign at the end and multiply by -1 each digit.
    LoadI -1
    StoreS sign
    LoadS num

digits_loop:
    ModI 10
    MulS sign # multiply by sign (-1 if negative or 1 if not)
    AddI '0'
    StoreSB buf len # stores digit in buf(5) at offset indicated by len(2)
    IncrS -1 len # len/idx by -1
    LoadS num # num
    DivI 10
    StoreS num # num
    JNZ digits_loop
done:
    LoadS sign # sign
    JPOS finish_str
    LoadI '-'
    StoreSB buf len # stores '-' in buf(5) at offset indicated by len(2)
    IncrS -1 len # len by -1
finish_str:
    LoadI 21
    SubS len
    StoreSB buf len # first byte of str8 is the length (to write)
    LoadS len # byte offset to find the start of the str8
    SysS write buf
    Return 6 # Unwind PC and 6 because of accumulator + 5 extra reserved stack entries
