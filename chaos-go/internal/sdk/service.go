package sdk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrVersionNotFound 表示请求切换到的版本目录不存在。
var ErrVersionNotFound = errors.New("sdk: target version not found")

// getSdkInfo 根据 SDK 类型推导版本列表与当前版本。
// repo：实时扫描 root 子目录；single：取目录名。绝对路径 == Current 标为启用。
func getSdkInfo(s SdkSource) SdkInfo {
	var info SdkInfo
	for _, it := range parseSources(s.Sources) {
		if it.Kind == "single" {
			abs := it.Root
			info.VersionList = append(info.VersionList, filepath.Base(abs))
			if abs == s.Current {
				info.CurrentVersion = filepath.Base(abs)
			}
			continue
		}
		// repo：扫描 root 子目录
		entries, err := os.ReadDir(it.Root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if name == linkName || name == versionFile {
				continue
			}
			abs := filepath.Join(it.Root, name)
			info.VersionList = append(info.VersionList, name)
			if abs == s.Current {
				info.CurrentVersion = name
			}
		}
	}
	return info
}

// SwitchSdkVersion 切换为指定版本：仅对 repo 来源生效，更新 symlink + .current-version + Current 绝对路径。
// 版本目录不存在返回 ErrVersionNotFound；软链/版本文件写入失败返回包装错误。
func SwitchSdkVersion(s SdkSource, version string) error {
	var targetRepo SdkSourceItem
	found := false
	for _, it := range parseSources(s.Sources) {
		if it.Kind != "repo" {
			continue
		}
		target := filepath.Join(it.Root, version)
		if _, err := os.Stat(target); err == nil {
			targetRepo = it
			found = true
			break
		}
	}
	if !found {
		return ErrVersionNotFound
	}

	target := filepath.Join(targetRepo.Root, version)
	link := filepath.Join(targetRepo.Root, linkName)
	verFile := filepath.Join(targetRepo.Root, versionFile)

	if _, err := os.Lstat(link); err == nil {
		os.Remove(link)
	}
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}
	if err := os.WriteFile(verFile, []byte(version), 0644); err != nil {
		return fmt.Errorf("write version file: %w", err)
	}

	return SetCurrentVersion(s.Name, target)
}
