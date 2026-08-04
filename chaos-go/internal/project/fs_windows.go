//go:build windows

package project

import (
	"os"
	"syscall"
	"time"
)

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

func filetimeToTime(ft syscall.Filetime) time.Time {
	nsec100 := int64(uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime))
	unixNano := (nsec100 - 116444736000000000) * 100
	return time.Unix(0, unixNano).UTC()
}
