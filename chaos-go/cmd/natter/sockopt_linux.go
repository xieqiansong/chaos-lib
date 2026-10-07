//go:build linux

package main

import "golang.org/x/sys/unix"

// setReuse 设置 SO_REUSEADDR + SO_REUSEPORT（Linux 需要后者让保活与 STUN socket 共享同一本地端口）。
func setReuse(fd uintptr) error {
	if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		return err
	}
	return unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
}

// bindToDevice 把 socket 绑定到指定网卡。
func bindToDevice(fd uintptr, iface string) error {
	return unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, iface)
}
