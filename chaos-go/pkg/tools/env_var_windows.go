//go:build windows

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func validScope(scope EnvScope) error {
	switch scope {
	case EnvScopeProcess, EnvScopeUser, EnvScopeMachine:
		return nil
	default:
		return fmt.Errorf("非法的作用域: %q，仅支持 Process/User/Machine", scope)
	}
}

func GetEnvVar(name string, scope EnvScope) (*EnvVar, error) {
	if err := validScope(scope); err != nil {
		return nil, err
	}
	const script = `
$val = [Environment]::GetEnvironmentVariable($env:CHAOS_ENV_NAME, $env:CHAOS_ENV_SCOPE)
[PSCustomObject]@{
    Name   = $env:CHAOS_ENV_NAME
    Value  = if ($null -eq $val) { '' } else { $val }
    Exists = ($null -ne $val)
    Scope  = $env:CHAOS_ENV_SCOPE
} | ConvertTo-Json -Compress
`
	t0 := time.Now()
	res, err := RunPowershell(context.Background(), script, ShellOpt{
		Env: []string{"CHAOS_ENV_NAME=" + name, "CHAOS_ENV_SCOPE=" + string(scope)},
	})
	fmt.Printf("[env_var_windows] GetEnvVar RunPowershell 耗时: %v\n", time.Since(t0))
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("读取环境变量失败: %s", strings.TrimSpace(res.Stderr))
	}
	var ev EnvVar
	t1 := time.Now()
	if err := json.Unmarshal([]byte(res.Stdout), &ev); err != nil {
		return nil, fmt.Errorf("解析 PowerShell 输出失败: %w", err)
	}
	fmt.Printf("[env_var_windows] GetEnvVar json.Unmarshal 耗时: %v\n", time.Since(t1))
	return &ev, nil
}

func GetEffectiveEnvVar(name string) (*EnvVar, error) {
	const script = `
$scopes = @('Process', 'User', 'Machine')
$found = $null
foreach ($s in $scopes) {
    $v = [Environment]::GetEnvironmentVariable($env:CHAOS_ENV_NAME, $s)
    if ($null -ne $v) {
        $found = [PSCustomObject]@{ Name = $env:CHAOS_ENV_NAME; Value = $v; Scope = $s; Exists = $true }
        break
    }
}
if ($null -eq $found) {
    $found = [PSCustomObject]@{ Name = $env:CHAOS_ENV_NAME; Value = ''; Scope = 'Process'; Exists = $false }
}
$found | ConvertTo-Json -Compress
`
	t0 := time.Now()
	res, err := RunPowershell(context.Background(), script, ShellOpt{
		Env: []string{"CHAOS_ENV_NAME=" + name},
	})
	fmt.Printf("[env_var_windows] GetEffectiveEnvVar RunPowershell 耗时: %v\n", time.Since(t0))
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("读取有效环境变量失败: %s", strings.TrimSpace(res.Stderr))
	}
	var ev EnvVar
	t1 := time.Now()
	if err := json.Unmarshal([]byte(res.Stdout), &ev); err != nil {
		return nil, fmt.Errorf("解析 PowerShell 输出失败: %w", err)
	}
	fmt.Printf("[env_var_windows] GetEffectiveEnvVar json.Unmarshal 耗时: %v\n", time.Since(t1))
	return &ev, nil
}

func SetEnvVar(name, value string, scope EnvScope) error {
	if err := validScope(scope); err != nil {
		return err
	}
	const script = `
[Environment]::SetEnvironmentVariable($env:CHAOS_ENV_NAME, $env:CHAOS_ENV_VALUE, $env:CHAOS_ENV_SCOPE)
`
	t0 := time.Now()
	res, err := RunPowershell(context.Background(), script, ShellOpt{
		Env: []string{
			"CHAOS_ENV_NAME=" + name,
			"CHAOS_ENV_VALUE=" + value,
			"CHAOS_ENV_SCOPE=" + string(scope),
		},
	})
	fmt.Printf("[env_var_windows] SetEnvVar RunPowershell 耗时: %v\n", time.Since(t0))
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("写入环境变量失败: %s", strings.TrimSpace(res.Stderr))
	}
	return nil
}

func RemoveEnvVar(name string, scope EnvScope) error {
	if err := validScope(scope); err != nil {
		return err
	}
	const script = `
[Environment]::SetEnvironmentVariable($env:CHAOS_ENV_NAME, $null, $env:CHAOS_ENV_SCOPE)
`
	t0 := time.Now()
	res, err := RunPowershell(context.Background(), script, ShellOpt{
		Env: []string{"CHAOS_ENV_NAME=" + name, "CHAOS_ENV_SCOPE=" + string(scope)},
	})
	fmt.Printf("[env_var_windows] RemoveEnvVar RunPowershell 耗时: %v\n", time.Since(t0))
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("删除环境变量失败: %s", strings.TrimSpace(res.Stderr))
	}
	return nil
}

func ListEnvVars(scope EnvScope) ([]EnvVar, error) {
	if err := validScope(scope); err != nil {
		return nil, err
	}
	const script = `
$d = [Environment]::GetEnvironmentVariables($env:CHAOS_ENV_SCOPE)
$list = $d.Keys | Sort-Object | ForEach-Object {
    [PSCustomObject]@{ Name = $_; Value = $d[$_] }
}
[PSCustomObject]@{ Vars = $list } | ConvertTo-Json -Compress
`
	t0 := time.Now()
	res, err := RunPowershell(context.Background(), script, ShellOpt{
		Env: []string{"CHAOS_ENV_SCOPE=" + string(scope)},
	})
	fmt.Printf("[env_var_windows] ListEnvVars RunPowershell 耗时: %v\n", time.Since(t0))
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("列出环境变量失败: %s", strings.TrimSpace(res.Stderr))
	}
	var out EnvVarList
	t1 := time.Now()
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		return nil, fmt.Errorf("解析 PowerShell 输出失败: %w", err)
	}
	fmt.Printf("[env_var_windows] ListEnvVars json.Unmarshal 耗时: %v\n", time.Since(t1))
	for i := range out.Vars {
		out.Vars[i].Scope = scope
		out.Vars[i].Exists = true
	}
	return out.Vars, nil
}

func IsElevated() bool {
	const script = `
([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
`
	res, err := RunPowershell(context.Background(), script, ShellOpt{})
	if err != nil || res.ExitCode != 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(res.Stdout), "True")
}

func GetHostname() string {
	const script = `$env:COMPUTERNAME`
	res, err := RunPowershell(context.Background(), script, ShellOpt{})
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

func GetUsername() string {
	const script = `$env:USERNAME`
	res, err := RunPowershell(context.Background(), script, ShellOpt{})
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}
