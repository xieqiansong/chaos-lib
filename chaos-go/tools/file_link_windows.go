//go:build windows

package tools

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

const (
	ioReparseTagMountPoint = 0xA0000003
	fsctlSetReparsePoint   = 0x000900A4
)

// CreateDirectoryLink 创建一个指向 targetDir 的目录映射。
// 在 Windows 上创建的是 Junction（目录联接点，基于 reparse point），
// 相比通用的目录符号链接（SYMLINKD）无需管理员权限，且系统开销更小。
// junctionPath 为新建的"链接"路径，targetDir 为实际指向的目录。
func CreateDirectoryLink(junctionPath, targetDir string) error {
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	// 统一使用反斜杠，并去掉尾部分隔符，保证与读取时一致。
	absTarget = strings.ReplaceAll(filepath.Clean(absTarget), "/", "\\")

	// 1. 先创建将成为 Junction 的空目录。
	if err := os.MkdirAll(junctionPath, 0); err != nil {
		return err
	}

	// 2. 以 reparse point 方式打开该目录。
	pathPtr, err := windows.UTF16PtrFromString(junctionPath)
	if err != nil {
		_ = os.Remove(junctionPath)
		return err
	}
	h, err := windows.CreateFile(
		pathPtr,
		windows.FILE_WRITE_ATTRIBUTES|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		_ = os.Remove(junctionPath)
		return err
	}
	defer windows.Close(h)

	// 3. 构造 reparse 数据并写入。
	data, err := junctionReparseData(absTarget)
	if err != nil {
		_ = os.Remove(junctionPath)
		return err
	}

	var bytesReturned uint32
	if err := windows.DeviceIoControl(
		h,
		fsctlSetReparsePoint,
		&data[0],
		uint32(len(data)),
		nil,
		0,
		&bytesReturned,
		nil,
	); err != nil {
		_ = os.Remove(junctionPath)
		return err
	}

	return nil
}

// junctionReparseData 构造 MOUNT_POINT 类型的 REPARSE_DATA_BUFFER。
// SubstituteName 必须以 "\??\" 前缀开头，PrintName 为可读路径。
// PathBuffer 中的两个宽字符串均以 null 结尾，Length 字段不含 null。
func junctionReparseData(target string) ([]byte, error) {
	substitute := `\??\` + target
	printName := target

	subUTF16 := utf16.Encode([]rune(substitute))
	printUTF16 := utf16.Encode([]rune(printName))

	subLen := len(subUTF16) * 2   // 不含 null 终止符的字节数
	printLen := len(printUTF16) * 2

	// PathBuffer 总长（含两个 null 终止符各 2 字节）。
	pathLen := subLen + 2 + printLen + 2
	// ReparseDataLength 为 Reserved 之后的数据长度：
	// 4 个 uint16 字段(8 字节) + PathBuffer(pathLen)。
	reparseDataLength := uint16(8 + pathLen)

	buf := make([]byte, 8+int(reparseDataLength))

	binary.LittleEndian.PutUint32(buf[0:4], ioReparseTagMountPoint)
	binary.LittleEndian.PutUint16(buf[4:6], reparseDataLength)
	binary.LittleEndian.PutUint16(buf[6:8], 0) // Reserved

	// MountPointReparseBuffer 字段
	binary.LittleEndian.PutUint16(buf[8:10], 0)                  // SubstituteNameOffset
	binary.LittleEndian.PutUint16(buf[10:12], uint16(subLen))        // SubstituteNameLength（不含 null）
	binary.LittleEndian.PutUint16(buf[12:14], uint16(subLen+2))     // PrintNameOffset（跳过 sub + null）
	binary.LittleEndian.PutUint16(buf[14:16], uint16(printLen))     // PrintNameLength（不含 null）

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
