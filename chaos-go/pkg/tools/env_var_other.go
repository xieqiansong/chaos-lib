//go:build !windows

package tools

import (
	"fmt"
	"os"
	"sort"
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
	if scope != EnvScopeProcess {
		return nil, fmt.Errorf("非 Windows 平台仅支持 Process 作用域，%q 暂不支持", scope)
	}
	v, ok := os.LookupEnv(name)
	return &EnvVar{Name: name, Value: v, Scope: EnvScopeProcess, Exists: ok}, nil
}

func GetEffectiveEnvVar(name string) (*EnvVar, error) {
	return &EnvVar{Name: name, Value: os.Getenv(name), Scope: EnvScopeProcess}, nil
}

func SetEnvVar(name, value string, scope EnvScope) error {
	if err := validScope(scope); err != nil {
		return err
	}
	if scope != EnvScopeProcess {
		return fmt.Errorf("非 Windows 平台仅支持 Process 作用域，%q 暂不支持", scope)
	}
	return os.Setenv(name, value)
}

func RemoveEnvVar(name string, scope EnvScope) error {
	if err := validScope(scope); err != nil {
		return err
	}
	if scope != EnvScopeProcess {
		return fmt.Errorf("非 Windows 平台仅支持 Process 作用域，%q 暂不支持", scope)
	}
	return os.Unsetenv(name)
}

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

func IsElevated() bool { return false }

func GetHostname() string { return "" }

func GetUsername() string { return "" }

func BroadcastEnvironmentChange() error { return nil }
