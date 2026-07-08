package tools

import (
	"chaos-lib/models"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	hwndBroadcast   = uintptr(0xFFFF)
	wmSettingChange = uintptr(0x001A)
	smtoAbortIfHung = 0x0002
	smtoNormal      = 0x0000
	regSz           = 1
	regExpandSz     = 2

	hkeyCurrentUser  = uintptr(0x80000001)
	hkeyLocalMachine = uintptr(0x80000002)
	keyRead          = 0x20019
	keyWrite         = 0x20006
	keyAllAccess     = 0xF003F
)

var (
	procRegOpenKeyExW       = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegOpenKeyExW")
	procRegQueryValueExW    = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegQueryValueExW")
	procRegSetValueExW      = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegSetValueExW")
	procRegEnumValueW       = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegEnumValueW")
	procRegDeleteValueW     = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegDeleteValueW")
	procRegCloseKey         = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegCloseKey")
	procSendMessageTimeoutW = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")
	procGetComputerNameW    = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetComputerNameW")
	procGetUserNameW        = windows.NewLazySystemDLL("advapi32.dll").NewProc("GetUserNameW")
	procOpenProcessToken    = windows.NewLazySystemDLL("advapi32.dll").NewProc("OpenProcessToken")
	procGetTokenInformation = windows.NewLazySystemDLL("advapi32.dll").NewProc("GetTokenInformation")
	procGetCurrentProcess   = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentProcess")
	procCloseHandle         = windows.NewLazySystemDLL("kernel32.dll").NewProc("CloseHandle")
)

const (
	envSubKeySystem = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	envSubKeyUser   = `Environment`
	tokenElevation  = 20
)

func isElevatedImpl() bool {
	var token windows.Token
	procHandle, _, _ := procGetCurrentProcess.Call()
	r1, _, _ := procOpenProcessToken.Call(
		procHandle,
		uintptr(0x0008),
		uintptr(unsafe.Pointer(&token)),
	)
	if r1 == 0 {
		return false
	}
	defer procCloseHandle.Call(uintptr(token))

	var elevation struct {
		TokenIsElevated int32
	}
	var returned uint32
	r1, _, _ = procGetTokenInformation.Call(
		uintptr(token),
		uintptr(tokenElevation),
		uintptr(unsafe.Pointer(&elevation)),
		unsafe.Sizeof(elevation),
		uintptr(unsafe.Pointer(&returned)),
	)
	return r1 != 0 && elevation.TokenIsElevated != 0
}

func getHostnameImpl() string {
	buf := make([]uint16, 256)
	size := uint32(len(buf))
	r1, _, _ := procGetComputerNameW.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func getUsernameImpl() string {
	buf := make([]uint16, 256)
	size := uint32(len(buf))
	r1, _, _ := procGetUserNameW.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func regOpenKey(hKey uintptr, subKey string, access uint32) (uintptr, error) {
	subKey16, err := windows.UTF16PtrFromString(subKey)
	if err != nil {
		return 0, err
	}
	var result uintptr
	r1, _, _ := procRegOpenKeyExW.Call(
		hKey,
		uintptr(unsafe.Pointer(subKey16)),
		0,
		uintptr(access),
		uintptr(unsafe.Pointer(&result)),
	)
	if r1 != 0 {
		return 0, fmt.Errorf("RegOpenKeyExW 失败: 代码=%d", r1)
	}
	return result, nil
}

func regCloseKey(hKey uintptr) {
	procRegCloseKey.Call(hKey)
}

func regReadAllValues(hKey uintptr) (map[string]string, error) {
	result := make(map[string]string)
	var nameBuf [256]uint16
	var valueBuf [32768]uint16

	for i := uint32(0); ; i++ {
		nameLen := uint32(len(nameBuf))
		valueLen := uint32(len(valueBuf) * 2)
		var valueType uint32

		r1, _, _ := procRegEnumValueW.Call(
			hKey,
			uintptr(i),
			uintptr(unsafe.Pointer(&nameBuf[0])),
			uintptr(unsafe.Pointer(&nameLen)),
			0,
			uintptr(unsafe.Pointer(&valueType)),
			uintptr(unsafe.Pointer(&valueBuf[0])),
			uintptr(unsafe.Pointer(&valueLen)),
		)
		if r1 != 0 {
			break
		}
		name := windows.UTF16ToString(nameBuf[:nameLen])
		if valueType == regSz || valueType == regExpandSz {
			value := windows.UTF16ToString(valueBuf[:valueLen/2])
			result[name] = value
		}
	}
	return result, nil
}

func regSetValue(hKey uintptr, name, value string, expandable bool) error {
	name16, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	value16, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return err
	}
	byteLen := uint32((len(value) + 1) * 2)
	valueType := uint32(regSz)
	if expandable {
		valueType = regExpandSz
	}
	if strings.Contains(value, "%") {
		valueType = regExpandSz
	}
	r1, _, _ := procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(name16)),
		0,
		uintptr(valueType),
		uintptr(unsafe.Pointer(value16)),
		uintptr(byteLen),
	)
	if r1 != 0 {
		return fmt.Errorf("RegSetValueExW 失败: 代码=%d, name=%s", r1, name)
	}
	return nil
}

func regDeleteValue(hKey uintptr, name string) error {
	name16, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	r1, _, _ := procRegDeleteValueW.Call(
		hKey,
		uintptr(unsafe.Pointer(name16)),
	)
	if r1 != 0 {
		return fmt.Errorf("RegDeleteValueW 失败: 代码=%d, name=%s", r1, name)
	}
	return nil
}

func readScopeFromRegistry(scope models.EnvScope) (map[string]string, error) {
	var root uintptr
	var subKey string
	switch scope {
	case models.EnvScopeSystem:
		root = hkeyLocalMachine
		subKey = envSubKeySystem
	case models.EnvScopeUser:
		root = hkeyCurrentUser
		subKey = envSubKeyUser
	default:
		return nil, fmt.Errorf("未知的 scope: %s", string(scope))
	}

	hKey, err := regOpenKey(root, subKey, keyRead)
	if err != nil {
		return nil, fmt.Errorf("打开 %s 注册表失败: %w", scope, err)
	}
	defer regCloseKey(hKey)

	return regReadAllValues(hKey)
}

func writeScopeToRegistry(scope models.EnvScope, desired map[string]string, current map[string]string) error {
	var root uintptr
	var subKey string
	switch scope {
	case models.EnvScopeSystem:
		root = hkeyLocalMachine
		subKey = envSubKeySystem
	case models.EnvScopeUser:
		root = hkeyCurrentUser
		subKey = envSubKeyUser
	default:
		return fmt.Errorf("未知的 scope: %s", string(scope))
	}

	hKey, err := regOpenKey(root, subKey, keyWrite)
	if err != nil {
		return fmt.Errorf("写 %s 注册表失败: %w", scope, err)
	}
	defer regCloseKey(hKey)

	var firstErr error
	for k, v := range desired {
		oldVal, existed := current[k]
		if existed && oldVal == v {
			continue
		}
		if err := regSetValue(hKey, k, v, false); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	for k := range current {
		if _, ok := desired[k]; !ok {
			if err := regDeleteValue(hKey, k); err != nil {
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	return firstErr
}

func broadcastEnvironmentChangeImpl() error {
	settingChangeStr, _ := windows.UTF16PtrFromString("Environment")
	var result uintptr
	r1, _, _ := procSendMessageTimeoutW.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(settingChangeStr)),
		uintptr(smtoAbortIfHung|smtoNormal),
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
	if r1 == 0 {
		return fmt.Errorf("SendMessageTimeoutW 失败")
	}
	return nil
}

func ReadAllEnvFromSystem() (*models.EnvSnapshot, error) {
	sysVars, err := readScopeFromRegistry(models.EnvScopeSystem)
	if err != nil {
		sysVars = map[string]string{}
	}
	userVars, err2 := readScopeFromRegistry(models.EnvScopeUser)
	if err2 != nil {
		userVars = map[string]string{}
	}

	snap := &models.EnvSnapshot{
		Meta: models.EnvMeta{
			Hostname: getHostnameImpl(),
			Username: getUsernameImpl(),
		},
		System: sysVars,
		User:   userVars,
	}

	if snap.System == nil {
		snap.System = map[string]string{}
	}
	if snap.User == nil {
		snap.User = map[string]string{}
	}

	if err != nil || err2 != nil {
		var combined error
		if err != nil {
			combined = err
		} else {
			combined = err2
		}
		return snap, combined
	}
	return snap, nil
}

func WriteAllEnvToSystem(snap *models.EnvSnapshot) (warnings []string, err error) {
	warnings = []string{}

	originalSystem, errSysRead := readScopeFromRegistry(models.EnvScopeSystem)
	if errSysRead != nil {
		originalSystem = map[string]string{}
	}
	originalUser, errUserRead := readScopeFromRegistry(models.EnvScopeUser)
	if errUserRead != nil {
		originalUser = map[string]string{}
	}

	systemVars := cloneMap(snap.System)
	userVars := cloneMap(snap.User)

	elevated := isElevatedImpl()
	if elevated {
		if err := writeScopeToRegistry(models.EnvScopeSystem, systemVars, originalSystem); err != nil {
			warnings = append(warnings, fmt.Sprintf("系统级变量部分写入失败: %v", err))
		}
	} else {
		warnings = append(warnings, "未以管理员身份运行，系统级变量未写入")
	}

	if err := writeScopeToRegistry(models.EnvScopeUser, userVars, originalUser); err != nil {
		warnings = append(warnings, fmt.Sprintf("用户级变量写入失败: %v", err))
	}

	if bcErr := broadcastEnvironmentChangeImpl(); bcErr != nil {
		warnings = append(warnings, fmt.Sprintf("广播环境变更失败: %v", bcErr))
	}

	return warnings, nil
}

func cloneMap(m map[string]string) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

var _ = syscall.Errno(0)
