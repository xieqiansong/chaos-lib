//go:build !windows

package envvar

func ReadAllEnvFromSystem() (*EnvSnapshot, error) {
	snap := &EnvSnapshot{
		System: map[string]string{},
		User:   map[string]string{},
	}
	return snap, nil
}

func WriteAllEnvToSystem(snap *EnvSnapshot) ([]string, error) {
	warnings := []string{"当前平台暂不支持写入环境变量（仅 Windows）"}
	return warnings, nil
}
