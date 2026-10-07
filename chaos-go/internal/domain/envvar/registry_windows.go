//go:build windows

package envvar

import (
	"fmt"
	"unsafe"

	"chaos-go/pkg/tools"
	"golang.org/x/sys/windows"
)

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

// domainScopeToTools 将领域层作用域（system/user）映射为 tools 包的作用域（Machine/User）。
// 领域层的 system 对应用户变量注册表项 HKLM\...\Environment，即 .NET 的 Machine 作用域；
// user 对应 HKCU\Environment，即 .NET 的 User 作用域。
func domainScopeToTools(scope EnvScope) (tools.EnvScope, bool) {
	switch scope {
	case EnvScopeSystem:
		return tools.EnvScopeMachine, true
	case EnvScopeUser:
		return tools.EnvScopeUser, true
	default:
		return "", false
	}
}

func isElevated() bool {
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

func getHostname() string {
	buf := make([]uint16, 256)
	size := uint32(len(buf))
	r1, _, _ := procGetComputerNameW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r1 == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func getUsername() string {
	buf := make([]uint16, 256)
	size := uint32(len(buf))
	r1, _, _ := procGetUserNameW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r1 == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// readScopeViaTools 通过 tools 包读取某一作用域下的全部环境变量，转为 map。
func readScopeViaTools(scope EnvScope) (map[string]string, error) {
	ts, ok := domainScopeToTools(scope)
	if !ok {
		return nil, fmt.Errorf("未知作用域: %s", string(scope))
	}
	vars, err := tools.ListEnvVars(ts)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(vars))
	for _, v := range vars {
		out[v.Name] = v.Value
	}
	return out, nil
}

// applyScope 将期望变量集合落盘到指定作用域：仅对变化项做 Set，对删除项做 Remove。
// 返回本次作用域内的警告（非致命错误）列表。
func applyScope(scope EnvScope, desired, current map[string]string) []string {
	ts, ok := domainScopeToTools(scope)
	if !ok {
		return []string{fmt.Sprintf("未知作用域: %s", string(scope))}
	}
	var warnings []string
	for k, v := range desired {
		if oldVal, existed := current[k]; existed && oldVal == v {
			continue
		}
		if e := tools.SetEnvVar(k, v, ts); e != nil {
			warnings = append(warnings, fmt.Sprintf("设置 %s 失败: %v", k, e))
		}
	}
	for k := range current {
		if _, ok := desired[k]; !ok {
			if e := tools.RemoveEnvVar(k, ts); e != nil {
				warnings = append(warnings, fmt.Sprintf("删除 %s 失败: %v", k, e))
			}
		}
	}
	return warnings
}

func broadcastEnvironmentChange() error {
	settingChangeStr, _ := windows.UTF16PtrFromString("Environment")
	var result uintptr
	r1, _, _ := procSendMessageTimeoutW.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(settingChangeStr)), uintptr(smtoAbortIfHung|smtoNormal), 5000, uintptr(unsafe.Pointer(&result)))
	if r1 == 0 {
		return fmt.Errorf("SendMessageTimeoutW 失败")
	}
	return nil
}

func ReadAllEnvFromSystem() (*EnvSnapshot, error) {
	sysVars, errSys := readScopeViaTools(EnvScopeSystem)
	if errSys != nil {
		sysVars = map[string]string{}
	}
	userVars, errUser := readScopeViaTools(EnvScopeUser)
	if errUser != nil {
		userVars = map[string]string{}
	}
	snap := &EnvSnapshot{
		Meta:   EnvMeta{Hostname: getHostname(), Username: getUsername()},
		System: sysVars,
		User:   userVars,
	}
	if snap.System == nil {
		snap.System = map[string]string{}
	}
	if snap.User == nil {
		snap.User = map[string]string{}
	}
	if errSys != nil || errUser != nil {
		var combined error
		if errSys != nil {
			combined = errSys
		} else {
			combined = errUser
		}
		return snap, combined
	}
	return snap, nil
}

func WriteAllEnvToSystem(snap *EnvSnapshot) (warnings []string, err error) {
	warnings = []string{}
	originalSystem, _ := readScopeViaTools(EnvScopeSystem)
	if originalSystem == nil {
		originalSystem = map[string]string{}
	}
	originalUser, _ := readScopeViaTools(EnvScopeUser)
	if originalUser == nil {
		originalUser = map[string]string{}
	}
	if isElevated() {
		if w := applyScope(EnvScopeSystem, cloneMap(snap.System), originalSystem); len(w) > 0 {
			warnings = append(warnings, fmt.Sprintf("系统级变量部分写入失败: %v", w))
		}
	} else {
		warnings = append(warnings, "未以管理员身份运行，系统级变量未写入")
	}
	if w := applyScope(EnvScopeUser, cloneMap(snap.User), originalUser); len(w) > 0 {
		warnings = append(warnings, fmt.Sprintf("用户级变量写入失败: %v", w))
	}
	if bcErr := broadcastEnvironmentChange(); bcErr != nil {
		warnings = append(warnings, fmt.Sprintf("广播环境变更失败: %v", bcErr))
	}
	return warnings, nil
}
