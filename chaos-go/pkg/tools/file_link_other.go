//go:build !windows

package tools

import "os"

// GetLinkInfo 读取路径的链接信息（非 Windows 用 os.Lstat / os.Readlink）。
// 路径不存在 / 无权限时返回 error；普通文件或目录正常返回（IsLink=false）。
func GetLinkInfo(path string) (*LinkInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	li := &LinkInfo{
		Path:       path,
		Attributes: info.Mode().String(),
		Mode:       info.Mode().String(),
	}
	if info.Mode()&os.ModeSymlink != 0 {
		li.IsLink = true
		li.LinkType = "SymbolicLink"
		if target, rerr := os.Readlink(path); rerr == nil {
			li.Target = target
		}
	}
	return li, nil
}

// CreateJunction 非 Windows 平台不支持 Junction，退化为创建目录符号链接。
func CreateJunction(linkPath, targetPath string) error {
	return os.Symlink(targetPath, linkPath)
}
