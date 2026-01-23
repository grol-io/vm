package main

import "syscall"

func main() {
	_, _ = syscall.Write(1, []byte("Hello, world!\n"))
	syscall.Exit(42)
}
