package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// ShellResult 保存命令执行的标准输出、标准错误与退出码。
// 即使命令以非零退出码结束，也不算 error（通过 ExitCode 判断），
// 仅在命令无法启动 / 被超时终止 / 进程缺失等情况下返回 error。
type ShellResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ShellOpt 配置单次命令执行的选项。
type ShellOpt struct {
	Dir     string        // 工作目录，空则继承当前进程
	Env     []string      // 附加环境变量（KEY=VALUE），会合并当前进程环境
	Timeout time.Duration // 超时，<=0 表示不限制
}

// RunCmd 执行系统 shell 命令。
// Windows 下通过 `cmd /c` 执行；其它平台通过 `sh -c` 执行。
// command 为完整命令行字符串（如 `dir /s` 或 `ls -la`）。
func RunCmd(ctx context.Context, command string, opt ShellOpt) (*ShellResult, error) {
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/c"
	}
	return run(ctx, shell, []string{flag, command}, opt)
}

// RunPowershell 执行一段 PowerShell 脚本。
// Windows 下调用 powershell.exe；其它平台若 PATH 中存在 pwsh 亦可运行。
// script 为整段 PowerShell 脚本文本，内部引号无需额外转义。
func RunPowershell(ctx context.Context, script string, opt ShellOpt) (*ShellResult, error) {
	bin := "powershell"
	if runtime.GOOS != "windows" {
		bin = "pwsh"
	}
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}
	return run(ctx, bin, args, opt)
}

// RunPwsh 执行一段 PowerShell 7 (pwsh) 脚本。
// 与 RunPowershell 语义相同，但固定使用 pwsh（PowerShell 7），
// 相比 Windows PowerShell 5.1（powershell.exe）冷启动明显更快。
// 要求运行环境已安装 PowerShell 7，且 pwsh 位于 PATH 中。
func RunPwsh(ctx context.Context, script string, opt ShellOpt) (*ShellResult, error) {
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}
	return run(ctx, "pwsh", args, opt)
}

// run 统一执行逻辑：处理超时、工作目录、环境变量与退出码。
func run(ctx context.Context, name string, args []string, opt ShellOpt) (*ShellResult, error) {
	if opt.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opt.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	if opt.Dir != "" {
		if info, statErr := os.Stat(opt.Dir); statErr != nil || !info.IsDir() {
			return &ShellResult{}, fmt.Errorf("工作目录不存在或不是目录: %s", opt.Dir)
		}
		cmd.Dir = opt.Dir
	}
	if len(opt.Env) > 0 {
		cmd.Env = append(os.Environ(), opt.Env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	res := &ShellResult{}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// 命令正常启动并以非零码退出：不算 error，交由调用方按 ExitCode 判断
			res.ExitCode = exitErr.ExitCode()
			res.Stdout = stdout.String()
			res.Stderr = stderr.String()
			return res, nil
		}
		// 进程无法启动 / 被超时终止 / 可执行文件缺失等：回填已捕获的部分输出
		res.Stdout = stdout.String()
		res.Stderr = stderr.String()
		return res, fmt.Errorf("%s 执行失败: %w", name, err)
	}

	res.ExitCode = 0
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()
	return res, nil
}
