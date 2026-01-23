; same as hello.go but in our assembly language
.const STDOUT 1
    Sys Write8 STDOUT msg # write to STDOUT (1)
    sys exit 42
msg:
    str8 "Hello World!\n"
