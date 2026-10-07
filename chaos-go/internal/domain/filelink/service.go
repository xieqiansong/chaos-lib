package filelink

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chaos-go/pkg/tools"
)

// 启停用法的失败原因（调用方据此区分 400 与 500）。
var (
	ErrAlreadyEnabled  = errors.New("文件连接已启用")
	ErrAlreadyDisabled = errors.New("文件连接已禁用")
	ErrSourceMissing   = errors.New("源路径不存在")
	ErrTargetExists    = errors.New("目标路径已存在，请先删除或移动")
)

// ── 派生状态 ────────────────────────────────────────────────────

// checkLinkStatus 按文件系统真实状态推导连接状态（不落库、每次读时重算）：
// normal 正常 / missing 目标缺失 / none 未启用 / invalid 无效 / conflict 冲突。
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
	// 直接取联接点真实指向（tools.GetLinkInfo.Target 已规范化，无需再脱 \??\ 前缀）。
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

// ── 用例 ────────────────────────────────────────────────────────

// ValidateForCreate 创建前校验：源 / 目标路径必填且源路径存在。
func ValidateForCreate(l *FileLink) error {
	if strings.TrimSpace(l.SourcePath) == "" || strings.TrimSpace(l.TargetPath) == "" {
		return errors.New("源路径和目标路径不能为空")
	}
	if _, err := os.Stat(l.SourcePath); err != nil {
		return fmt.Errorf("源路径不存在: %s", l.SourcePath)
	}
	return nil
}

// CleanupLink 记录软删后清理联接点；联接点已不存在视为清理成功（幂等）。
func CleanupLink(l *FileLink) error {
	if !l.Status {
		return nil
	}
	if err := os.Remove(l.TargetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理联接点失败: %w", err)
	}
	return nil
}

// SetStatus 启停联接点：先做磁盘副作用（失败则不动库），再落库状态；
// 落库失败时撤掉刚建的联接点，避免「库里未启用、磁盘上却已建」的不一致。
// 返回的哨兵错误（ErrAlreadyXxx / ErrSourceMissing / ErrTargetExists）由调用方映射为 400。
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
		// tools.CreateJunction 的参数为 (linkPath, targetPath)，与 domain 历史的
		// (target, junction) 顺序相反，此处对调：TargetPath 是新建联接点，SourcePath 是真实目录。
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
		// 联接点已删除且不可复原，只能如实上报。
		return fmt.Errorf("状态更新失败: %w", err)
	}
	l.Status = false
	return nil
}

// ToggleStatus 按 ID 切换联接点状态：先取未软删的实体，再执行带副作用的启停。
// 供通用启停路由 crud.RegisterToggle 使用，编排留在用例层，handler 只做配置与转调。
func ToggleStatus(id int, enable bool) (any, error) {
	link, err := findActiveByID(id)
	if err != nil {
		return nil, err
	}
	if err := SetStatus(link, enable); err != nil {
		return nil, err
	}
	return toOne(link), nil
}
