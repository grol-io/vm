; itoa: convert the number in `num` to decimal using loops, store into a str8 word, and print it.
; Builds digits least-significant-first with ModI/DivI 10, then prefixes length byte.
; Handles negative numbers including min_int64 (once we increase buf to more than 1 word/7chars).

; Simple case to debug:
;    LoadI -47
;    CALL itoa
;    Sys Exit 0 ; for now to shorten the current debug
; Rest/normal tests:

    LoadI -1
    ShiftI 63 ; -9223372036854775808
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

itoa: ; prints accumulator as a decimal string
    Var num sign idx _ _ buf ; -> Push 5 reserve 5 additional entries on stack
    ; Maximum length + sign + \n (for numbers in the order of min_int64) including room for the length byte
    ; We decrement so the bytes are placed in reverse order of modulo operations and thus in the right order
    ; in a single pass.
    LoadI 21
    StoreS idx
    ; Add the newline
    LoadI '\n'
    StoreSB buf idx ; stores newline in buf at offset indicated by idx
    IncrS -1 idx
    LoadI 1
    StoreS sign
    LoadS num
    JGT 0 digits_loop
    ; else mark/remember as negative to add the minus sign at the end and multiply by -1 each digit.
    LoadI -1
    StoreS sign

digits_loop:
    LoadI 10
    IdivS num ; num /= 10; A = num % 10
    MulS sign ; multiply by sign (-1 if negative or 1 if not)
    AddI '0'
    StoreSB buf idx ; stores digit in buf at offset indicated by idx
    IncrS -1 idx ; decrement idx by 1 (which thus also increments the length=21-idx)
    LoadS num
    JNE 0 digits_loop
done:
    LoadS sign ; sign
    JGT 0 finish_str
    LoadI '-'
    StoreSB buf idx ; stores '-' in buf at offset indicated by idx
    IncrS -1 idx ; idx by -1
finish_str:
    LoadI 21 ; calculate length based on what we started idx at
    SubS idx
    StoreSB buf idx ; first byte of str8 is the length (to write)
    LoadS idx ; byte offset to find the start of the str8
    SysS write buf
    Return ; -> Ret 6 to pop the 6 (`var`s) and return address to PC
