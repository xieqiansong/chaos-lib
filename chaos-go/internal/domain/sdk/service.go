package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrVersionNotFound 表示请求切换到的版本目录不存在。
var ErrVersionNotFound = errors.New("sdk: target version not found")

// ── 校验 ────────────────────────────────────────────────────────

// ValidateSources 校验来源数组：kind 必须为 repo / single，root 必填。
func ValidateSources(raw json.RawMessage) error {
	for _, it := range parseSources(raw) {
		if it.Kind != "repo" && it.Kind != "single" {
			return &InvalidInputError{Msg: "Source kind must be 'repo' or 'single'"}
		}
		if it.Root == "" {
			return &InvalidInputError{Msg: "Source root is required"}
		}
	}
	return nil
}

// ── 用例 ────────────────────────────────────────────────────────

// ListSources 列出所有未删除的 SDK 类型。
func ListSources() ([]SdkSource, error) {
	return ListActiveSdkSources()
}

// SdkVersions 返回所有启用类型的版本信息，map key = 类型 Name。
func SdkVersions() (map[string]SdkInfo, error) {
	srcs, err := ListActiveSdkSources()
	if err != nil {
		return nil, err
	}
	result := make(map[string]SdkInfo)
	for _, s := range srcs {
		if !s.Enabled {
			continue
		}
		result[s.Name] = getSdkInfo(s)
	}
	return result, nil
}

// SdkVersion 返回指定类型的版本信息。
func SdkVersion(typ string) (*SdkInfo, error) {
	s, err := FindSdkSource(typ)
	if err != nil {
		return nil, err
	}
	info := getSdkInfo(*s)
	return &info, nil
}

// SwitchVersion 切换指定类型的版本（仅 repo 来源生效）。
func SwitchVersion(typ, version string) error {
	s, err := FindSdkSource(typ)
	if err != nil {
		return err
	}
	return switchSdkVersion(*s, version)
}

// CreateSource 登记一个 SDK 类型：名称必填、来源合法、同名不重复。
func CreateSource(src *SdkSource) error {
	if src.Name == "" {
		return ErrNameRequired
	}
	if err := ValidateSources(src.Sources); err != nil {
		return err
	}
	count, err := CountActiveByName(src.Name)
	if err != nil {
		return err
	}
	if count > 0 {
		return &SourceExistsError{Name: src.Name}
	}
	return CreateSdkSourceRow(src)
}

// UpdateSource 按字段映射更新 SDK 类型，返回更新后的记录。
func UpdateSource(name string, patch SourcePatch) (*SdkSource, error) {
	s, err := FindSdkSource(name)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if patch.Sources != nil && len(*patch.Sources) > 0 {
		if err := ValidateSources(*patch.Sources); err != nil {
			return nil, err
		}
		updates["sources"] = *patch.Sources
	}
	if patch.Current != nil && *patch.Current != "" {
		updates["current"] = *patch.Current
	}
	if patch.Enabled != nil {
		updates["enabled"] = *patch.Enabled
	}
	if patch.Note != nil {
		updates["note"] = *patch.Note
	}
	if len(updates) == 0 {
		return s, nil
	}
	if err := ApplySdkSourcePatch(name, updates); err != nil {
		return nil, err
	}
	// 回读最新，避免把更新前的快照返回给前端。
	return FindSdkSource(name)
}

// DeleteSource 软删除指定 SDK 类型。
func DeleteSource(name string) error {
	if _, err := FindSdkSource(name); err != nil {
		return err
	}
	return SoftDeleteSdkSource(name)
}

// ── 内部实现 ────────────────────────────────────────────────────

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

// switchSdkVersion 切换为指定版本：仅对 repo 来源生效，更新 symlink + .current-version + Current 绝对路径。
// 版本目录不存在返回 ErrVersionNotFound；软链 / 版本文件写入失败返回包装错误。
func switchSdkVersion(s SdkSource, version string) error {
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
