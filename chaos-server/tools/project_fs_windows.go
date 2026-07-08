//go:build windows

package tools

import (
	"os"
	"syscall"
	"time"
)

// DirCreatedAt 返回目录的创建时间（Windows 下取文件创建时间 / birth time）。
// 无法获取时返回零值，由调用方回退为当前时间。
func DirCreatedAt(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	wfs, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}
	}
	return filetimeToTime(wfs.CreationTime)
}

// filetimeToTime 将 Windows FILETIME（自 1601-01-01 起的 100 纳秒计数）转为 time.Time（UTC）。
func filetimeToTime(ft syscall.Filetime) time.Time {
	nsec100 := int64(uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime))
	unixNano := (nsec100 - 116444736000000000) * 100
	return time.Unix(0, unixNano).UTC()
}
