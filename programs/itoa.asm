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
    Push 5 # reserve 5 additional entries on stack: num:0 + sign:1, len/idx:2, buf:3,4,5
    LoadI 21
    StoreS 2 # len
    # Add the newline
    LoadI '\n'
    StoreSB 5 2 # stores newline in buf(5) at offset indicated by len(2)
    IncrS -1 2 # len/idx by -1
    LoadI 1
    StoreS 1 # sign
    LoadS 0 # num
    JPOS digits_loop
    # else mark/remember as negative to add the minus sign at the end and multiply by -1 each digit.
    LoadI -1
    StoreS 1 # sign
    LoadS 0 # num

digits_loop:
    ModI 10
    MulS 1 # multiply by sign (-1 if negative or 1 if not)
    AddI '0'
    StoreSB 5 2 # stores digit in buf(5) at offset indicated by len(2)
    IncrS -1 2 # len/idx by -1
    LoadS 0 # num
    DivI 10
    StoreS 0 # num
    JNZ digits_loop
done:
    LoadS 1 # sign
    JPOS finish_str
    LoadI '-'
    StoreSB 5 2 # stores '-' in buf(5) at offset indicated by len(2)
    IncrS -1 2 # len by -1
finish_str:
    LoadI 21
    SubS 2 # len
    StoreSB 5 2 # store at buf(5) with byte offset indicated by len(2)
    LoadS 2 # len offset
    SysS write 5 # buf
    Return 6 # Unwind PC and 6 because of accumulator + 5 extra reserved stack entries
