//go:build !windows

package project

import (
	"os"
	"time"
)

func DirCreatedAt(path string) time.Time {
	if _, err := os.Stat(path); err != nil {
		return time.Time{}
	}
	return time.Time{}
}
