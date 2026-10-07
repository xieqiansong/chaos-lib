//go:build windows

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetLinkInfo 读取路径的链接信息（Windows 下通过 PowerShell Get-Item）。
// 路径不存在 / 无权限时返回 error；普通文件或目录正常返回（IsLink=false）。
// 路径通过环境变量 CHAOS_LINK_PATH 传入，避免路径中含空格 / 中文 / 引号时的转义问题。
func GetLinkInfo(path string) (*LinkInfo, error) {
	const script = `
$item = Get-Item -LiteralPath $env:CHAOS_LINK_PATH -Force
[PSCustomObject]@{
    FullName   = $item.FullName
    LinkType   = if ($null -ne $item.LinkType) { $item.LinkType.ToString() } else { '' }
    Target     = if ($null -ne $item.Target) { ($item.Target | ForEach-Object { $_ }) -join ';' } else { '' }
    IsLink     = [bool]($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)
    Attributes = $item.Attributes.ToString()
    Mode       = $item.Mode
} | ConvertTo-Json -Compress
`
	res, err := RunPowershell(context.Background(), script, ShellOpt{
		Env: []string{"CHAOS_LINK_PATH=" + path},
	})
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("查询链接信息失败: %s", strings.TrimSpace(res.Stderr))
	}
	var info LinkInfo
	if err := json.Unmarshal([]byte(res.Stdout), &info); err != nil {
		return nil, fmt.Errorf("解析 PowerShell 输出失败: %w", err)
	}
	return &info, nil
}

// CreateJunction 通过 PowerShell `New-Item -ItemType Junction` 创建目录联接点（Junction）。
// linkPath 本身不能已存在（已存在则报错）；其多级父目录不存在时会自动创建。
// targetPath 为目标文件夹。路径均通过环境变量传入，规避空格 / 中文 / 引号转义。
func CreateJunction(linkPath, targetPath string) error {
	// 已存在则报错，避免静默覆盖已有路径
	if _, err := os.Lstat(linkPath); err == nil {
		return fmt.Errorf("链接路径已存在: %s", linkPath)
	}
	// 确保父目录存在：New-Item 不会自动创建多级父目录
	if parent := filepath.Dir(linkPath); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("创建父目录失败: %v", err)
		}
	}
	const script = `
New-Item -ItemType Junction -Path $env:CHAOS_LINK_PATH -Target $env:CHAOS_TARGET_PATH | Out-Null
`
	res, err := RunPowershell(context.Background(), script, ShellOpt{
		Env: []string{"CHAOS_LINK_PATH=" + linkPath, "CHAOS_TARGET_PATH=" + targetPath},
	})
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("创建 Junction 失败: %s", strings.TrimSpace(res.Stderr))
	}
	return nil
}
