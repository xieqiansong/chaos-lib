//go:build !windows

package filelink

import (
	"errors"
	"os"
)

func CreateJunction(target, junction string) error {
	return errors.New("junction 仅支持 Windows 平台")
}

func RemoveJunction(path string) error {
	return os.RemoveAll(path)
}

func IsJunction(path string) bool {
	return false
}
