//go:build !windows

package tools

import (
	"chaos-go/models"
)

func ReadAllEnvFromSystem() (*models.EnvSnapshot, error) {
	snap := &models.EnvSnapshot{
		System: map[string]string{},
		User:   map[string]string{},
	}
	return snap, nil
}

func WriteAllEnvToSystem(snap *models.EnvSnapshot) ([]string, error) {
	warnings := []string{"当前平台暂不支持写入环境变量（仅 Windows）"}
	return warnings, nil
}
