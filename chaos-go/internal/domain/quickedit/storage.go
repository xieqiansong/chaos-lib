package quickedit

import (
	"fmt"
	"os"
)

var ErrEnvNotReady = fmt.Errorf("quickedit: 环境变量读写回调未初始化")

func readContent(file *QuickEditFile) (content string, sizeBytes int, err error) {
	if isEnvVirtualFile(file.FilePath) {
		if EnvReadContent == nil {
			return "", 0, ErrEnvNotReady
		}
		content, size, err := EnvReadContent()
		if err != nil {
			return "", 0, fmt.Errorf("读取环境变量失败: %w", err)
		}
		return content, size, nil
	}
	data, err := os.ReadFile(file.FilePath)
	if err != nil {
		return "", 0, fmt.Errorf("读取文件失败: %w", err)
	}
	return string(data), len(data), nil
}

func writeContent(file *QuickEditFile, content string) (warnings []string, err error) {
	if isEnvVirtualFile(file.FilePath) {
		if EnvWriteContent == nil {
			return nil, ErrEnvNotReady
		}
		w, writeErr := EnvWriteContent(content)
		if writeErr != nil {
			w = append(w, fmt.Sprintf("写入异常: %v", writeErr))
		}
		return w, nil
	}
	if err := os.WriteFile(file.FilePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败 (可能需要管理员权限): %w", err)
	}
	return nil, nil
}

func readDiskFile(path string) (data []byte, err error) {
	data, err = os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return data, nil
}
