; wc.asm: like `wc -l`, counts newline characters in files
; Usage: wc [file1] [file2] ...
; If no arguments, reads from stdin.

.const STDIN 0
.const STDOUT 1
.const STDERR 2
; reserve and read 256 bytes at a time
.const maxBytes 256
; so for buffer; 32 words * 8 bytes = 256 bytes
.const sizeInWords maxBytes/8
.const O_RDONLY 0


    Pop 0 ; argc
    JGT 0 no_stdin
    ; stdin case:
    LoadI STDIN
    Push 0 ; fd_param
    Call count_lines
    Call itoa
    Sys Exit 0
no_stdin:
    StoreR argc_val
    StoreR num_files
arg_loop:
    LoadS 0 ; top of stack is now the filename address
    Sys Write8 STDOUT 0
    Sys Write8 STDOUT space_str
    LoadS 0
    ; Open file
    ; Sys Open flags path
    Sys Open O_RDONLY 0 ; flags=O_RDONLY(0), path=0(Accumulator)
    JLT 0 sys_error
    Push 0; fd_param

    Call count_lines

    StoreR cur_count
    LoadR total_count
    AddR cur_count
    StoreR total_count ; accumulate total count
    ; Close file
    Pop 0; fd_param
    Sys Close 0
    JNE 0 sys_error
    ; Print current file count
    LoadR cur_count
    Call itoa
    Pop 0 ; pop current_file
    LoadR argc_val
    IncrR -1 argc_val
    JGT 0 arg_loop
    ; Print total count if more than 1 file
    LoadR num_files
    JEQ 1 no_total_print
    Sys Write8 STDOUT total_label
    LoadR total_count
    Call itoa
no_total_print:
    Sys Exit 0

sys_error:
    Sys Write8 STDOUT stdout_err_msg
    Sys Write8 STDERR err_msg
    Pop 0
    Sys Write8 STDERR 0 ; filename from A
    Sys Write8 STDERR newline_str
    Sys Exit 1

; count_lines subroutine
; reads from fd_var (stack param) until EOF
; returns line count in accumulator
count_lines:
    LoadI 0 ; initialize count to 0
    ; Allocate buffer and vars on stack.
    Var count bytes_read buf[sizeInWords]
    Param fd_var ; input param, setup for SysS ReadF
read_loop:
    LoadI maxBytes
    SysS ReadF fd_var buf
    JLTE 0 check_result
    ; Process bytes (accumulator has bytes_read)
    StoreS bytes_read
count_loop:
    IncrS -1 bytes_read
    LoadSB buf bytes_read
    JNE '\n' not_newline
    IncrS 1 count ; found newline, count it
not_newline:
    LoadS bytes_read
    JGT 0 count_loop
    JumpR read_loop
check_result:
    JNE 0 sys_error ; error case, abort
    ; else EOF case, return count
    LoadS count
    Return; Will generate ret 35 (cleanup stack variables (32 + 3 = 35 words))

; Memory storage
argc_val:
    data 0
num_files: ; same but not decremented
    data 0
total_count:
    data 0
cur_count:
    data 0
space_str:
    str8 " "
newline_str:
    str8 "\n"
err_msg:
    str8 "File error: "
stdout_err_msg:
    str8 "...err...\n"
total_label:
    str8 "total: "
