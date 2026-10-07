//go:build !windows

package tools

import (
	"fmt"
	"os"
	"sort"
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

// GetEnvVar 读取环境变量（非 Windows 仅支持 Process 作用域，User/Machine 无跨平台标准，直接报错）。
func GetEnvVar(name string, scope EnvScope) (*EnvVar, error) {
	if err := validScope(scope); err != nil {
		return nil, err
	}
	if scope != EnvScopeProcess {
		return nil, fmt.Errorf("非 Windows 平台仅支持 Process 作用域，%q 暂不支持", scope)
	}
	v, ok := os.LookupEnv(name)
	return &EnvVar{Name: name, Value: v, Scope: EnvScopeProcess, Exists: ok}, nil
}

// GetEffectiveEnvVar 读取进程内当前生效的环境变量（os.Getenv）。
func GetEffectiveEnvVar(name string) (*EnvVar, error) {
	return &EnvVar{Name: name, Value: os.Getenv(name), Scope: EnvScopeProcess}, nil
}

// SetEnvVar 写入环境变量（非 Windows 仅支持 Process 作用域，仅影响当前进程）。
func SetEnvVar(name, value string, scope EnvScope) error {
	if err := validScope(scope); err != nil {
		return err
	}
	if scope != EnvScopeProcess {
		return fmt.Errorf("非 Windows 平台仅支持 Process 作用域，%q 暂不支持", scope)
	}
	return os.Setenv(name, value)
}

// RemoveEnvVar 删除环境变量（非 Windows 仅支持 Process 作用域，幂等）。
func RemoveEnvVar(name string, scope EnvScope) error {
	if err := validScope(scope); err != nil {
		return err
	}
	if scope != EnvScopeProcess {
		return fmt.Errorf("非 Windows 平台仅支持 Process 作用域，%q 暂不支持", scope)
	}
	return os.Unsetenv(name)
}

// ListEnvVars 列出当前进程的所有环境变量（非 Windows 仅支持 Process 作用域）。
func ListEnvVars(scope EnvScope) ([]EnvVar, error) {
	if err := validScope(scope); err != nil {
		return nil, err
	}
	if scope != EnvScopeProcess {
		return nil, fmt.Errorf("非 Windows 平台仅支持 Process 作用域，%q 暂不支持", scope)
	}
	env := os.Environ()
	vars := make([]EnvVar, 0, len(env))
	for _, kv := range env {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				vars = append(vars, EnvVar{
					Name:  kv[:i],
					Value: kv[i+1:],
					Scope: EnvScopeProcess,
				})
				break
			}
		}
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
	return vars, nil
}

// ── 系统级 Windows 原语的跨平台占位（非 Windows 不提供真实实现）──

// IsElevated 非 Windows 平台恒为 false。
func IsElevated() bool { return false }

// GetHostname 非 Windows 平台返回空字符串。
func GetHostname() string { return "" }

// GetUsername 非 Windows 平台返回空字符串。
func GetUsername() string { return "" }

// BroadcastEnvironmentChange 非 Windows 平台为空操作。
func BroadcastEnvironmentChange() error { return nil }
