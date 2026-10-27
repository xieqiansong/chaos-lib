//go:build !windows

package tools

import "os"

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

func CreateJunction(linkPath, targetPath string) error {
	return os.Symlink(targetPath, linkPath)
}
