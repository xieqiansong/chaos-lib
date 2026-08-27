package project

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
func RemoveDirSafe(path string) error {
	return forceRemoveAll(path)
}

func forceRemoveAll(path string) error {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if _, err := os.Stat(path); err != nil {
			return nil
		}
		clearReadOnly(path)
		if runtime.GOOS == "windows" {
			lastErr = rmdirWindows(path)
		} else {
			lastErr = os.RemoveAll(path)
		}
		if _, err := os.Stat(path); err != nil {
			return nil
		}
		time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("目录删除后仍然存在，可能有进程（编辑器 / git fsmonitor / 杀毒软件）占用: %s", path)
	}
	return lastErr
}

func clearReadOnly(root string) {
	_ = filepath.WalkDir(root, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		_ = os.Chmod(p, 0o700)
		return nil
	})
}

func rmdirWindows(path string) error {
	_, _ = exec.Command("cmd", "/c", "attrib", "-R", "-H", "-S", "/S", "/D", path).CombinedOutput()
	out, err := exec.Command("cmd", "/c", "rmdir", "/s", "/q", path).CombinedOutput()
	if err != nil {
		if _, err := os.Stat(path); err != nil {
			return nil
		}
		return fmt.Errorf("rmdir 失败: %v, 输出: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func MoveProjectFolder(oldAbs, newAbs string) error {
	if oldAbs == newAbs {
		return nil
	}
	if info, err := os.Stat(oldAbs); err != nil || !info.IsDir() {
		// 源目录不存在（可能已被手动/并发删除）：幂等返回成功，
		// 使删除/移动流程可继续完成数据库元数据更新，避免接口报错
		return nil
	}
	if _, err := os.Stat(newAbs); err == nil {
		return fmt.Errorf("目标目录已存在: %s", newAbs)
	}
	if parent := filepath.Dir(newAbs); parent != "" {
		if mkErr := os.MkdirAll(parent, 0o755); mkErr != nil {
			return fmt.Errorf("创建目标父目录失败: %v", mkErr)
		}
	}
	// 同卷优先原子移动（O(1)，不复制数据）
	if err := os.Rename(oldAbs, newAbs); err == nil {
		return nil
	}
	// 跨卷回退：复制 + 删源
	if cerr := copyDir(oldAbs, newAbs); cerr != nil {
		_ = RemoveDirSafe(newAbs)
		return fmt.Errorf("移动目录失败: %v", cerr)
	}
	_ = RemoveDirSafe(oldAbs)
	return nil
}

func MoveToRecycleBin(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("读取路径信息失败: %v", err)
		}
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
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("送回收站后原目录仍残留（可能 .git 被进程占用）: %s", path)
		}
		return nil
	}
	return RemoveDirSafe(path)
}

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
