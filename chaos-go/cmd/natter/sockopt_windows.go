//go:build windows

package main

import (
	"fmt"
	"syscall"
)

// setReuse 设置 SO_REUSEADDR。
// Windows 的 SO_REUSEADDR 语义即允许多个 socket 复用同一端口，无需 SO_REUSEPORT。
func setReuse(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}

// bindToDevice 在 Windows 上不支持。
func bindToDevice(fd uintptr, iface string) error {
	return fmt.Errorf("binding to an interface is not supported on your platform: %s", iface)
}
