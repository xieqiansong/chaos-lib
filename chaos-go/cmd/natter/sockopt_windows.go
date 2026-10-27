//go:build windows

package main

import (
	"fmt"
	"syscall"
)

func setReuse(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}

func bindToDevice(fd uintptr, iface string) error {
	return fmt.Errorf("binding to an interface is not supported on your platform: %s", iface)
}
