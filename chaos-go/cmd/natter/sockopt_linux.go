//go:build linux

package main

import "golang.org/x/sys/unix"

// setReuse 设置 SO_REUSEADDR + SO_REUSEPORT。
//
// Natter 需要让多个 socket 同时绑定同一个本地端口（保活 socket 与 STUN socket 并存），
// 因此 Linux 下必须开 SO_REUSEPORT。
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
