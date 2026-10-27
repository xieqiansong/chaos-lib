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

type ShellResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type ShellOpt struct {
	Dir     string
	Env     []string
	Timeout time.Duration
}

func RunCmd(ctx context.Context, command string, opt ShellOpt) (*ShellResult, error) {
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/c"
	}
	return run(ctx, shell, []string{flag, command}, opt)
}

func RunPowershell(ctx context.Context, script string, opt ShellOpt) (*ShellResult, error) {
	bin := "powershell"
	if runtime.GOOS != "windows" {
		bin = "pwsh"
	}
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}
	return run(ctx, bin, args, opt)
}

func RunPwsh(ctx context.Context, script string, opt ShellOpt) (*ShellResult, error) {
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}
	return run(ctx, "pwsh", args, opt)
}

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

			res.ExitCode = exitErr.ExitCode()
			res.Stdout = stdout.String()
			res.Stderr = stderr.String()
			return res, nil
		}

		res.Stdout = stdout.String()
		res.Stderr = stderr.String()
		return res, fmt.Errorf("%s 执行失败: %w", name, err)
	}

	res.ExitCode = 0
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()
	return res, nil
}
