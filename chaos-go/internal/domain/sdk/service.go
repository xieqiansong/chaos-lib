package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrVersionNotFound = errors.New("sdk: target version not found")

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

func ListSources() ([]SdkSource, error) {
	return ListActiveSdkSources()
}

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

func SdkVersion(typ string) (*SdkInfo, error) {
	s, err := FindSdkSource(typ)
	if err != nil {
		return nil, err
	}
	info := getSdkInfo(*s)
	return &info, nil
}

func SwitchVersion(typ, version string) error {
	s, err := FindSdkSource(typ)
	if err != nil {
		return err
	}
	return switchSdkVersion(*s, version)
}

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

	return FindSdkSource(name)
}

func DeleteSource(name string) error {
	if _, err := FindSdkSource(name); err != nil {
		return err
	}
	return SoftDeleteSdkSource(name)
}

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
