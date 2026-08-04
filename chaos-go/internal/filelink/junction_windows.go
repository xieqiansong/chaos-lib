//go:build windows

package filelink

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func CreateJunction(target, junction string) error {
	_ = os.MkdirAll(filepath.Dir(junction), 0o755)
	if err := os.Mkdir(junction, 0o700); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("创建 junction 目录失败: %v", err)
		}
	}
	jp, err := syscall.UTF16PtrFromString(filepath.Clean(junction))
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(jp,
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		_ = os.Remove(junction)
		return fmt.Errorf("打开 junction 目标失败: %v", err)
	}
	defer windows.CloseHandle(handle)

	substituteName := []byte(`\??\` + target)
	reparseBuf := makeReparseBuffer(substituteName)
	var bytesReturned uint32
	err = windows.DeviceIoControl(
		handle,
		windows.FSCTL_SET_REPARSE_POINT,
		&reparseBuf[0],
		uint32(len(reparseBuf)),
		nil, 0,
		&bytesReturned,
		nil,
	)
	if err != nil {
		_ = os.Remove(junction)
		return fmt.Errorf("设置 junction 失败: %v", err)
	}
	return nil
}

func RemoveJunction(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	jp, err := syscall.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(jp,
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(handle)

	// ReparseTag (4 bytes) + ReparseDataLength (2 bytes) + Reserved (2 bytes)
	repBuf := make([]byte, windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	binary.LittleEndian.PutUint32(repBuf[0:4], windows.IO_REPARSE_TAG_MOUNT_POINT)
	repBuf[4] = 0
	repBuf[5] = 0
	repBuf[6] = 0
	repBuf[7] = 0

	var bytesReturned uint32
	err = windows.DeviceIoControl(
		handle,
		windows.FSCTL_DELETE_REPARSE_POINT,
		&repBuf[0],
		uint32(len(repBuf)),
		nil, 0,
		&bytesReturned,
		nil,
	)
	if err != nil {
		return fmt.Errorf("删除 junction 失败: %v", err)
	}
	return os.Remove(path)
}

func IsJunction(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	// 进一步检查是否为 junction（而非普通 symlink）
	jp, err := syscall.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return false
	}
	handle, err := windows.CreateFile(jp,
		0,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	var bytesReturned uint32
	buf := make([]byte, windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	err = windows.DeviceIoControl(
		handle,
		windows.FSCTL_GET_REPARSE_POINT,
		nil, 0,
		&buf[0], uint32(len(buf)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return false
	}
	return true
}

// mountPointReparseBuffer 是 MountPointReparseBuffer 的原始布局
type mountPointReparseBuffer struct {
	SubstituteNameOffset uint16
	SubstituteNameLength uint16
	PrintNameOffset      uint16
	PrintNameLength      uint16
	PathBuffer           [1]uint16
}

type reparseDataBuffer struct {
	ReparseTag        uint32
	ReparseDataLength uint16
	Reserved          uint16
	MountPoint        mountPointReparseBuffer
}

func makeReparseBuffer(substituteName []byte) []byte {
	var buf [windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE]byte
	rd := (*reparseDataBuffer)(unsafe.Pointer(&buf[0]))
	rd.ReparseTag = windows.IO_REPARSE_TAG_MOUNT_POINT
	subLen := uint16(len(substituteName))
	printLen := subLen
	if printLen > 0 {
		printLen -= 2
	}
	rd.MountPoint.SubstituteNameLength = subLen
	rd.MountPoint.SubstituteNameOffset = 0
	rd.MountPoint.PrintNameLength = printLen
	rd.MountPoint.PrintNameOffset = subLen + 2
	pathBufOffset := unsafe.Offsetof(rd.MountPoint.PathBuffer)
	copy(buf[pathBufOffset:], substituteName)
	copy(buf[pathBufOffset+uintptr(subLen)+2:], substituteName[4:])
	dataLen := uint32(pathBufOffset) + uint32(subLen+2) + uint32(len(substituteName)-4)
	return buf[:dataLen]
}
