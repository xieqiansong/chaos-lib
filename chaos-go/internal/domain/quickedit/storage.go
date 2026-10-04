package quickedit

import (
	"fmt"
	"os"
)

// 本文件是文件内容的读写实现（基础设施层）：
//   - 受管控文件可能是「环境变量虚拟文件」，其读写走 envvar 注入的回调
//   - 其余为磁盘文件，直接读写
//
// service 只与这里的函数打交道，不直接 os.ReadFile / os.WriteFile。

// ErrEnvNotReady 虚拟文件读写回调尚未注入（启动未完成）。
var ErrEnvNotReady = fmt.Errorf("quickedit: 环境变量读写回调未初始化")

// readContent 读取受管控文件的当前内容。
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

// writeContent 写入受管控文件的内容，返回非致命告警。
// 虚拟文件写入出错不视为失败（内容可能部分生效），告警交由调用方呈现。
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

// readDiskFile 读取磁盘上任意路径的内容（登记文件前用，不受管控列表约束）。
func readDiskFile(path string) (data []byte, err error) {
	data, err = os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return data, nil
}
