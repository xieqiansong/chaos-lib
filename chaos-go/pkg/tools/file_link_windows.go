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

func CreateJunction(linkPath, targetPath string) error {

	if _, err := os.Lstat(linkPath); err == nil {
		return fmt.Errorf("链接路径已存在: %s", linkPath)
	}

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
