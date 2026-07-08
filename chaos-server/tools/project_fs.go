package tools

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DirExists 判断目录是否存在
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FileExists 判断路径（文件或目录）是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// MkdirAllSafe 递归创建目录（便于登记一个尚未在磁盘上存在的项目组根目录）
func MkdirAllSafe(path string) error {
	return os.MkdirAll(path, 0o755)
}

// RemoveDirSafe 删除目录（用于删除项目时可选的物理移除）。
// Windows 下用 PowerShell Remove-Item -Recurse -Force 确保 .git 中的只读文件被可靠删除；
// 非 Windows 平台使用 os.RemoveAll。
func RemoveDirSafe(path string) error {
	if !FileExists(path) {
		return nil
	}
	if runtime.GOOS == "windows" {
		return removeDirWindows(path)
	}
	return os.RemoveAll(path)
}

// removeDirWindows 暴力删除 Windows 目录（专门对付 .git 这种带只读/隐藏/系统属性的顽固目录）。
// 步骤：1) 递归遍历所有文件/目录 2) 剥光属性（只读、隐藏、系统全清为 Normal）
//  3. PowerShell Remove-Item -Recurse -Force 执行删除
//
// 账号需要管理员权限。
func removeDirWindows(path string) error {
	escaped := strings.ReplaceAll(path, "'", "''")
	ps := fmt.Sprintf(
		"$ErrorActionPreference='Stop'; "+
			"$target='%s'; "+
			// 递归获取所有项目（-Force 包含隐藏和系统文件），对每个项目剥光属性
			"$items=Get-ChildItem -LiteralPath $target -Recurse -Force -ErrorAction SilentlyContinue; "+
			"if($items){ foreach($i in $items){ try{$i.Attributes='Normal'}catch{} } }; "+
			// 目录本身也剥光属性
			"try{(Get-Item -LiteralPath $target -Force).Attributes='Normal'}catch{}; "+
			// 现在可以安心删除
			"Remove-Item -LiteralPath $target -Recurse -Force -ErrorAction Stop",
		escaped,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除目录失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// MoveProjectFolder 将源目录递归复制到目标目录（仅复制，不删除源）。
// 采用「先复制、后删源」的安全策略：由调用方在数据库更新成功后再将原目录送入回收站。
// 安全检查：
//   - 源目录必须存在
//   - 目标目录必须不存在（避免覆盖）
func MoveProjectFolder(oldAbs, newAbs string) error {
	if oldAbs == newAbs {
		return nil
	}
	if !DirExists(oldAbs) {
		return fmt.Errorf("源目录不存在: %s", oldAbs)
	}
	if FileExists(newAbs) {
		return fmt.Errorf("目标目录已存在: %s", newAbs)
	}

	// 确保目标父目录存在
	if parent := filepath.Dir(newAbs); parent != "" {
		if mkErr := os.MkdirAll(parent, 0o755); mkErr != nil {
			return fmt.Errorf("创建目标父目录失败: %v", mkErr)
		}
	}

	// 递归拷贝源目录到目标
	if cerr := copyDir(oldAbs, newAbs); cerr != nil {
		// 清理可能已部分拷贝的目标，避免脏数据
		_ = RemoveDirSafe(newAbs)
		return fmt.Errorf("复制目录失败: %v", cerr)
	}
	return nil
}

// MoveToRecycleBin 将路径（文件或目录）移动到系统回收站（安全删除）。
// Windows 下通过 PowerShell 调用 VB FileSystem 送入回收站并抑制确认对话框；
// 其他平台回退为直接删除（os.RemoveAll）。
func MoveToRecycleBin(path string) error {
	if !FileExists(path) {
		return nil
	}

	if runtime.GOOS == "windows" {
		escaped := strings.ReplaceAll(path, "'", "''")
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("读取路径信息失败: %v", err)
		}
		verb := "DeleteFile"
		if info.IsDir() {
			verb = "DeleteDirectory"
		}
		ps := fmt.Sprintf(
			"Add-Type -AssemblyName Microsoft.VisualBasic; "+
				"[Microsoft.VisualBasic.FileIO.FileSystem]::%s('%s', 'OnlyErrorDialogs', 'SendToRecycleBin')",
			verb, escaped,
		)
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("送入回收站失败: %v (%s)", err, string(out))
		}
		return nil
	}

	return RemoveDirSafe(path)
}

// copyDir 递归拷贝目录（含文件与子目录）
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()|0o700); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if cerr := copyDir(srcPath, dstPath); cerr != nil {
				return cerr
			}
			continue
		}

		if cerr := copyFile(srcPath, dstPath); cerr != nil {
			return cerr
		}
	}
	return nil
}

// copyFile 拷贝单个文件（保留文件权限）
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
