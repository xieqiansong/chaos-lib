package filelink

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chaos-go/pkg/tools"
)

var (
	ErrAlreadyEnabled  = errors.New("文件连接已启用")
	ErrAlreadyDisabled = errors.New("文件连接已禁用")
	ErrSourceMissing   = errors.New("源路径不存在")
	ErrTargetExists    = errors.New("目标路径已存在，请先删除或移动")
)

func checkLinkStatus(sourcePath, targetPath string, enabled bool) string {
	_, err := os.Lstat(targetPath)
	if err != nil {
		if enabled {
			return "missing"
		}
		return "none"
	}
	if !enabled {
		return "invalid"
	}

	info, err := tools.GetLinkInfo(targetPath)
	if err == nil && info.Target != "" {
		absActual, _ := filepath.Abs(info.Target)
		absSource, _ := filepath.Abs(sourcePath)
		if strings.EqualFold(absActual, absSource) {
			return "normal"
		}
	}
	return "conflict"
}

func GetLinkStatus(id int) (string, error) {
	link, err := findActiveByID(id)
	if err != nil {
		return "", err
	}
	return checkLinkStatus(link.SourcePath, link.TargetPath, link.Status), nil
}

func ValidateForCreate(l *FileLink) error {
	if strings.TrimSpace(l.SourcePath) == "" || strings.TrimSpace(l.TargetPath) == "" {
		return errors.New("源路径和目标路径不能为空")
	}
	if _, err := os.Stat(l.SourcePath); err != nil {
		return fmt.Errorf("源路径不存在: %s", l.SourcePath)
	}
	return nil
}

func SetStatus(l *FileLink, enable bool) error {
	if enable {
		if l.Status {
			return ErrAlreadyEnabled
		}
		if _, err := os.Stat(l.SourcePath); os.IsNotExist(err) {
			return ErrSourceMissing
		}
		if _, err := os.Lstat(l.TargetPath); err == nil {
			return ErrTargetExists
		}

		if err := tools.CreateJunction(l.TargetPath, l.SourcePath); err != nil {
			return fmt.Errorf("创建目录联接点失败: %w", err)
		}
		if err := updateStatus(l.ID, true); err != nil {
			if rmErr := os.Remove(l.TargetPath); rmErr != nil && !os.IsNotExist(rmErr) {
				return fmt.Errorf("状态更新失败: %v（且联接点清理失败: %v）", err, rmErr)
			}
			return fmt.Errorf("状态更新失败: %w", err)
		}
		l.Status = true
		return nil
	}

	if !l.Status {
		return ErrAlreadyDisabled
	}
	if err := os.Remove(l.TargetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除联接点失败: %w", err)
	}
	if err := updateStatus(l.ID, false); err != nil {

		return fmt.Errorf("状态更新失败: %w", err)
	}
	l.Status = false
	return nil
}

func ToggleStatus(id int, enable bool) (any, error) {
	link, err := findActiveByID(id)
	if err != nil {
		return nil, err
	}
	if err := SetStatus(link, enable); err != nil {
		return nil, err
	}
	return link, nil
}
