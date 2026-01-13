; sys_packing.asm
; Test generic SYS WRITE ID LEN ADDR
; ID=Write (check syscall.go, we added it at the end? or midway?)
; SyscallWriteString parses case-insensitive.
.const STDOUT 1
.const LEN 8

    ; Write 8 bytes from msg to STDOUT
    Sys Write STDOUT LEN msg
    Sys Exit 0

msg:
    ; "HiWorld\n" (8 bytes)
    ; H=0x48 i=0x69 W=0x57 o=0x6F r=0x72 l=0x6C d=0x64 \n=0x0A
    ; Packed into 64-bit word (Little Endian):
    ; 0A 64 6C 72 6F 57 69 48
    data 0x0A646C726F576948
