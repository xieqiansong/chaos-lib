package tools

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// RemoveDirSafe 删除目录（用于删除项目时可选的物理移除）。
// 专门针对 .git 这类「只读 + 隐藏 + 海量小文件」的顽固目录做了加固：
//  1. 递归清除只读属性（os.Chmod 在 Windows 下等价于清除 READONLY 位）
//  2. Windows 用 attrib 清 R/H/S 属性后再 rmdir /s /q（不受 PowerShell -Recurse 竞态 bug 影响）
//  3. 整体带重试，规避杀毒/文件监听造成的瞬时占用
//
// 注意：若 .git 下有文件被其它进程（编辑器、git fsmonitor、杀毒等）持有句柄，
// 任何删除方式都无法删除该子树，此时返回错误而非静默残留。
func RemoveDirSafe(path string) error {
	return forceRemoveAll(path)
}

// forceRemoveAll 强力递归删除，处理只读属性与瞬时占用（重试）。
func forceRemoveAll(path string) error {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if _, err := os.Stat(path); err != nil {
			return nil
		}
		// 每次尝试前都先清一遍只读属性（可能有新文件出现或上次未清干净）
		clearReadOnly(path)

		if runtime.GOOS == "windows" {
			lastErr = rmdirWindows(path)
		} else {
			lastErr = os.RemoveAll(path)
		}

		if _, err := os.Stat(path); err != nil {
			return nil
		}
		// 目录仍在：可能被瞬时占用，退避后重试
		time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("目录删除后仍然存在，可能有进程（编辑器 / git fsmonitor / 杀毒软件）占用: %s", path)
	}
	return lastErr
}

// clearReadOnly 递归清除只读属性（Windows 下清除 READONLY，其它平台补齐写权限）。
func clearReadOnly(root string) {
	_ = filepath.WalkDir(root, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return nil // 尽力而为，忽略单个失败
		}
		_ = os.Chmod(p, 0o700)
		return nil
	})
}

// rmdirWindows 用 attrib 清属性 + rmdir /s /q 删除目录。
// rmdir /s /q 是 Windows 上最可靠的递归删除方式，不受 PowerShell Remove-Item -Recurse
// 「directory is not empty」竞态 bug 影响。
func rmdirWindows(path string) error {
	// -R 只读 -H 隐藏 -S 系统 /S 递归 /D 含目录本身
	_, _ = exec.Command("cmd", "/c", "attrib", "-R", "-H", "-S", "/S", "/D", path).CombinedOutput()

	out, err := exec.Command("cmd", "/c", "rmdir", "/s", "/q", path).CombinedOutput()
	if err != nil {
		if _, err := os.Stat(path); err != nil {
			return nil // rmdir 对已删除路径也会报错，实际已删则视为成功
		}
		return fmt.Errorf("rmdir 失败: %v, 输出: %s", err, strings.TrimSpace(string(out)))
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
	if info, err := os.Stat(oldAbs); err != nil || !info.IsDir() {
		return fmt.Errorf("源目录不存在: %s", oldAbs)
	}
	if _, err := os.Stat(newAbs); err == nil {
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
	if _, err := os.Stat(path); err != nil {
		return nil
	}

	if runtime.GOOS == "windows" {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("读取路径信息失败: %v", err)
		}
		// 先清只读属性，否则 .git 下的只读文件可能导致送回收站失败
		if info.IsDir() {
			clearReadOnly(path)
		} else {
			_ = os.Chmod(path, 0o700)
		}

		escaped := strings.ReplaceAll(path, "'", "''")
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
		// 回收站操作可能对被占用的子树静默跳过，校验是否真的删干净
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("送回收站后原目录仍残留（可能 .git 被进程占用）: %s", path)
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
