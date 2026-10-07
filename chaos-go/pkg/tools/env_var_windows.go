//go:build windows

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// validScope 校验作用域是否合法，非法时返回错误。
func validScope(scope EnvScope) error {
	switch scope {
	case EnvScopeProcess, EnvScopeUser, EnvScopeMachine:
		return nil
	default:
		return fmt.Errorf("非法的作用域: %q，仅支持 Process/User/Machine", scope)
	}
}

// GetEnvVar 读取指定作用域下的单个环境变量。
// 变量不存在时返回 Exists=false、Value="" 的 EnvVar 且 err=nil；
// 作用域非法时返回 error。路径 / 变量名均通过环境变量传入，规避空格与特殊字符转义。
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

// GetEffectiveEnvVar 读取变量的"有效值"：按 Process → User → Machine 优先级，
// 返回第一个存在的值对应的作用域（与进程内 [Environment]::GetEnvironmentVariable(name) 语义一致）。
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

// SetEnvVar 在指定作用域写入 / 覆盖环境变量（Machine 需管理员权限，否则报错）。
// value 为空字符串表示删除（等价于 RemoveEnvVar）。路径与变量名均通过环境变量传入。
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

// RemoveEnvVar 删除指定作用域下的环境变量（置为 null）。
// 作用域非法或变量本就不存在时均不报错（幂等）。
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

// ListEnvVars 列出指定作用域下的所有环境变量。
// scope 非法时返回 error。返回结果按变量名排序。
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

// ── 系统级 Windows 原语（供 envvar 领域层调用）────────────────────

const (
	hwndBroadcast   = uintptr(0xFFFF)
	wmSettingChange = uintptr(0x001A)
	smtoAbortIfHung = 0x0002
	smtoNormal      = 0x0000
	tokenElevation  = 20
)

var (
	procSendMessageTimeoutW = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")
	procGetComputerNameW    = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetComputerNameW")
	procGetUserNameW        = windows.NewLazySystemDLL("advapi32.dll").NewProc("GetUserNameW")
	procOpenProcessToken    = windows.NewLazySystemDLL("advapi32.dll").NewProc("OpenProcessToken")
	procGetTokenInformation = windows.NewLazySystemDLL("advapi32.dll").NewProc("GetTokenInformation")
	procGetCurrentProcess   = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentProcess")
	procCloseHandle         = windows.NewLazySystemDLL("kernel32.dll").NewProc("CloseHandle")
)

// IsElevated 判断当前进程是否以管理员（提权）身份运行。
func IsElevated() bool {
	var token windows.Token
	procHandle, _, _ := procGetCurrentProcess.Call()
	r1, _, _ := procOpenProcessToken.Call(procHandle, uintptr(0x0008), uintptr(unsafe.Pointer(&token)))
	if r1 == 0 {
		return false
	}
	defer procCloseHandle.Call(uintptr(token))
	var elevation struct{ TokenIsElevated int32 }
	var returned uint32
	r1, _, _ = procGetTokenInformation.Call(uintptr(token), uintptr(tokenElevation), uintptr(unsafe.Pointer(&elevation)), unsafe.Sizeof(elevation), uintptr(unsafe.Pointer(&returned)))
	return r1 != 0 && elevation.TokenIsElevated != 0
}

// GetHostname 返回计算机名。
func GetHostname() string {
	buf := make([]uint16, 256)
	size := uint32(len(buf))
	r1, _, _ := procGetComputerNameW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r1 == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// GetUsername 返回当前用户名。
func GetUsername() string {
	buf := make([]uint16, 256)
	size := uint32(len(buf))
	r1, _, _ := procGetUserNameW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r1 == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// BroadcastEnvironmentChange 广播 WM_SETTINGCHANGE（Environment），通知其它进程环境变量已变更。
func BroadcastEnvironmentChange() error {
	settingChangeStr, _ := windows.UTF16PtrFromString("Environment")
	var result uintptr
	r1, _, _ := procSendMessageTimeoutW.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(settingChangeStr)), uintptr(smtoAbortIfHung|smtoNormal), 5000, uintptr(unsafe.Pointer(&result)))
	if r1 == 0 {
		return fmt.Errorf("SendMessageTimeoutW 失败")
	}
	return nil
}
