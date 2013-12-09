// +build !windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

func isatty() bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, os.Stdin.Fd(), uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
}

func tty_ready() error {
	return nil
}

func tty_term() {
}
