package main

import "syscall"

func main() {
	_, _ = syscall.Write(1, []byte("Hello World!\n"))
	syscall.Exit(42)
}
