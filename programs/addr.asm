# Simple program to add two numbers using relative address based instructions
    LoadR num1
    AddR  num2
    ExitI 0
num1:
    Data -42
num2:
    Data 0x7F_FF_FF_FF_FF
