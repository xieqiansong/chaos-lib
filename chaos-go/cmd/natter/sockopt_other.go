//go:build !linux && !windows

package main

import (
	"fmt"
	"syscall"
)

func setReuse(fd uintptr) error {
	if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return err
	}
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
}

func bindToDevice(fd uintptr, iface string) error {
	return fmt.Errorf("binding to an interface is not supported on your platform: %s", iface)
}
