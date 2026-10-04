//go:build windows

package filelink

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"

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

	substituteName, err := makeReparseBuffer(target)
	if err != nil {
		_ = os.Remove(junction)
		return fmt.Errorf("构造 junction reparse 数据失败: %v", err)
	}
	var bytesReturned uint32
	err = windows.DeviceIoControl(
		handle,
		windows.FSCTL_SET_REPARSE_POINT,
		&substituteName[0],
		uint32(len(substituteName)),
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

// makeReparseBuffer 构造 MOUNT_POINT 类型的 REPARSE_DATA_BUFFER。
// SubstituteName 必须以 "\??\" 前缀开头，PrintName 为可读路径。
// PathBuffer 中的两个宽字符串均以 null 结尾，Length 字段不含 null。
func makeReparseBuffer(target string) ([]byte, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	// 统一使用反斜杠，并去掉尾部分隔符，保证与读取时一致。
	absTarget = strings.ReplaceAll(filepath.Clean(absTarget), "/", "\\")

	substitute := `\??\` + absTarget
	printName := absTarget

	subUTF16 := utf16.Encode([]rune(substitute))
	printUTF16 := utf16.Encode([]rune(printName))

	subLen := len(subUTF16) * 2 // 不含 null 终止符的字节数
	printLen := len(printUTF16) * 2

	// PathBuffer 总长（含两个 null 终止符各 2 字节）。
	pathLen := subLen + 2 + printLen + 2
	// ReparseDataLength 为 Reserved 之后的数据长度：
	// 4 个 uint16 字段(8 字节) + PathBuffer(pathLen)。
	reparseDataLength := uint16(8 + pathLen)

	buf := make([]byte, 8+int(reparseDataLength))

	binary.LittleEndian.PutUint32(buf[0:4], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buf[4:6], reparseDataLength)
	binary.LittleEndian.PutUint16(buf[6:8], 0) // Reserved

	// MountPointReparseBuffer 字段
	binary.LittleEndian.PutUint16(buf[8:10], 0)                 // SubstituteNameOffset
	binary.LittleEndian.PutUint16(buf[10:12], uint16(subLen))   // SubstituteNameLength（不含 null）
	binary.LittleEndian.PutUint16(buf[12:14], uint16(subLen+2)) // PrintNameOffset（跳过 sub + null）
	binary.LittleEndian.PutUint16(buf[14:16], uint16(printLen)) // PrintNameLength（不含 null）

	// PathBuffer: subUTF16 + null + printUTF16 + null
	off := 16
	for i, v := range subUTF16 {
		binary.LittleEndian.PutUint16(buf[off+i*2:], v)
	}
	base := off + subLen + 2 // 跳过 sub 及其 null 终止符（已为 0）
	for i, v := range printUTF16 {
		binary.LittleEndian.PutUint16(buf[base+i*2:], v)
	}
	// printUTF16 之后的 null 终止符由 buf 默认零值保证

	return buf, nil
}
