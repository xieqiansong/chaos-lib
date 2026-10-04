package envvar

import (
	"encoding/json"
	"fmt"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// ── 编解码（领域层，纯函数）─────────────────────────────────────

// MarshalEnvToTOML 把快照序列化为 TOML 文本（缺字段在此兜底，保证可解析）。
func MarshalEnvToTOML(snapshot *EnvSnapshot) (string, error) {
	if snapshot == nil {
		return "", fmt.Errorf("快照为空")
	}
	if snapshot.Meta.SavedAt == "" {
		snapshot.Meta.SavedAt = time.Now().Format(time.RFC3339)
	}
	if snapshot.System == nil {
		snapshot.System = map[string]string{}
	}
	if snapshot.User == nil {
		snapshot.User = map[string]string{}
	}
	data, err := toml.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("TOML序列化失败: %w", err)
	}
	return string(data), nil
}

// ParseEnvFromTOML 把 TOML 文本解析为快照。
func ParseEnvFromTOML(content string) (*EnvSnapshot, error) {
	var snap EnvSnapshot
	if err := toml.Unmarshal([]byte(content), &snap); err != nil {
		return nil, fmt.Errorf("TOML解析失败: %w", err)
	}
	if snap.System == nil {
		snap.System = map[string]string{}
	}
	if snap.User == nil {
		snap.User = map[string]string{}
	}
	return &snap, nil
}

// ApplySectionPatch 把一段增量补丁（Set / Unset / Path 增删改）应用到指定段落。
func ApplySectionPatch(section *EnvSection, patch *EnvSectionPatch) {
	if patch == nil {
		return
	}
	if *section == nil {
		*section = map[string]string{}
	}
	for k, v := range patch.Set {
		(*section)[k] = v
	}
	for _, k := range patch.Unset {
		delete(*section, k)
	}
	pathPatch := patch.Path
	if len(pathPatch.Replace) > 0 {
		(*section)["Path"] = joinNonEmpty(dedupedCopy(pathPatch.Replace), ";")
	} else if len(pathPatch.Prepend) > 0 || len(pathPatch.Append) > 0 || len(pathPatch.Remove) > 0 {
		existing := splitAndTrim((*section)["Path"], ";")
		removeSet := make(map[string]bool, len(pathPatch.Remove))
		for _, r := range pathPatch.Remove {
			removeSet[r] = true
		}
		result := make([]string, 0, len(existing)+len(pathPatch.Prepend)+len(pathPatch.Append))
		result = append(result, pathPatch.Prepend...)
		for _, item := range existing {
			if !removeSet[item] {
				result = append(result, item)
			}
		}
		result = append(result, pathPatch.Append...)
		(*section)["Path"] = joinNonEmpty(result, ";")
	}
}

// ── 用例 ───────────────────────────────────────────────────────

// EnsureVirtualFile 确保环境变量虚拟文件已登记；首次登记时用当前系统变量落一条初始快照。
func EnsureVirtualFile() (int, error) {
	file, err := findVirtualFile()
	if err != nil {
		return 0, err
	}
	if file != nil {
		return file.ID, nil
	}
	// 读取失败不阻断登记：先建文件，快照内容为空快照由调用方后续同步补齐。
	snap, _ := ReadAllEnvFromSystem()
	content, err := MarshalEnvToTOML(snap)
	if err != nil {
		return 0, err
	}
	created, err := createVirtualFile()
	if err != nil {
		return 0, fmt.Errorf("创建虚拟文件失败: %w", err)
	}
	if _, err := takeSnapshot(created.ID, content); err != nil {
		return 0, fmt.Errorf("写入初始快照失败: %w", err)
	}
	return created.ID, nil
}

// Load 读取当前系统环境变量，并附最近一次快照的 id 与时间。
func Load() (*EnvGetResponse, error) {
	fileID, err := EnsureVirtualFile()
	if err != nil {
		return nil, err
	}
	snap, readErr := ReadAllEnvFromSystem()
	if snap == nil {
		snap = &EnvSnapshot{}
	}
	resp := &EnvGetResponse{
		Meta:     snap.Meta,
		System:   snap.System,
		User:     snap.User,
		Warnings: []string{},
	}
	if readErr != nil {
		resp.Warnings = append(resp.Warnings, fmt.Sprintf("读取环境变量部分失败: %v", readErr))
	}
	if latest, err := latestSnapshot(fileID); err == nil && latest != nil {
		resp.SnapshotID = latest.ID
		resp.SnapshotTime = latest.CreatedAt.Format(time.RFC3339)
	}
	return resp, nil
}

// Sync 以当前系统变量为准重落一条快照（不改写系统环境变量）。
func Sync() ([]string, error) {
	fileID, err := EnsureVirtualFile()
	if err != nil {
		return nil, err
	}
	snap, readErr := ReadAllEnvFromSystem()
	if snap == nil {
		snap = &EnvSnapshot{}
	}
	content, err := MarshalEnvToTOML(snap)
	if err != nil {
		return nil, err
	}
	if _, err := takeSnapshot(fileID, content); err != nil {
		return nil, fmt.Errorf("快照失败: %w", err)
	}
	warnings := []string{}
	if readErr != nil {
		warnings = append(warnings, fmt.Sprintf("读取部分失败: %v", readErr))
	}
	return warnings, nil
}

// ApplyPatch 按增量补丁改写系统 / 用户两段变量，并落一条快照。
// 返回的 warnings 为写入与快照过程中的非致命告警（系统写入已生效）。
func ApplyPatch(req *EnvPatchRequest) ([]string, error) {
	fileID, err := EnsureVirtualFile()
	if err != nil {
		return nil, err
	}
	snap, readErr := ReadAllEnvFromSystem()
	if snap == nil {
		if readErr != nil {
			return nil, fmt.Errorf("读取环境变量失败: %w", readErr)
		}
		return nil, fmt.Errorf("读取环境变量失败")
	}
	ApplySectionPatch(&snap.System, req.System)
	ApplySectionPatch(&snap.User, req.User)

	warnings, writeErr := WriteAllEnvToSystem(snap)
	if writeErr != nil {
		warnings = append(warnings, fmt.Sprintf("写入异常: %v", writeErr))
	}
	content, err := MarshalEnvToTOML(snap)
	if err != nil {
		return warnings, err
	}
	if _, snapErr := takeSnapshot(fileID, content); snapErr != nil {
		warnings = append(warnings, fmt.Sprintf("快照失败: %v", snapErr))
	}
	return warnings, nil
}

// ReplaceAll 整体替换 system / user 两段变量（nil 表示该段不变），并落一条快照。
func ReplaceAll(req *EnvPutRequest) ([]string, error) {
	fileID, err := EnsureVirtualFile()
	if err != nil {
		return nil, err
	}
	current, _ := ReadAllEnvFromSystem()
	if current == nil {
		current = &EnvSnapshot{}
	}
	if req.System != nil {
		current.System = *req.System
		if current.System == nil {
			current.System = map[string]string{}
		}
	}
	if req.User != nil {
		current.User = *req.User
		if current.User == nil {
			current.User = map[string]string{}
		}
	}

	warnings, writeErr := WriteAllEnvToSystem(current)
	if writeErr != nil {
		warnings = append(warnings, fmt.Sprintf("写入异常: %v", writeErr))
	}
	content, err := MarshalEnvToTOML(current)
	if err != nil {
		return warnings, err
	}
	if _, snapErr := takeSnapshot(fileID, content); snapErr != nil {
		warnings = append(warnings, fmt.Sprintf("快照失败: %v", snapErr))
	}
	return warnings, nil
}

// GetSnapshotDetail 返回指定快照的内容与解析后的结构化视图；解析失败时退化为原文 + 错误信息。
func GetSnapshotDetail(snapID int) (*SnapshotDetail, error) {
	fileID, err := EnsureVirtualFile()
	if err != nil {
		return nil, err
	}
	snap, err := findSnapshot(fileID, snapID)
	if err != nil {
		return nil, err
	}
	detail := &SnapshotDetail{
		ID:         snap.ID,
		FileID:     snap.FileID,
		RawContent: snap.Content,
		CreatedAt:  snap.CreatedAt.Format(time.RFC3339),
		SizeBytes:  snap.SizeBytes,
	}
	parsed, parseErr := ParseEnvFromTOML(snap.Content)
	if parseErr != nil {
		detail.ParseError = parseErr.Error()
		return detail, nil
	}
	detail.Meta = &parsed.Meta
	detail.System = parsed.System
	detail.User = parsed.User
	return detail, nil
}

// ── 供 quickedit 虚拟文件读写回调使用 ───────────────────────────

// ReadVirtualContent 读系统环境变量并序列化为 TOML（quickedit 读取虚拟文件内容）。
func ReadVirtualContent() (content string, sizeBytes int, err error) {
	snap, err := ReadAllEnvFromSystem()
	if err != nil {
		return "", 0, err
	}
	content, tomlErr := MarshalEnvToTOML(snap)
	if tomlErr != nil {
		return "", 0, tomlErr
	}
	return content, len([]byte(content)), nil
}

// WriteVirtualContent 解析 TOML 并写入系统环境变量（quickedit 保存虚拟文件内容）。
func WriteVirtualContent(content string) (warnings []string, err error) {
	snap, err := ParseEnvFromTOML(content)
	if err != nil {
		return nil, err
	}
	return WriteAllEnvToSystem(snap)
}

// ── JSON 辅助（避免循环引用 reflect）───────────────────────────

func EnvSnapshotToJSON(snap *EnvSnapshot) ([]byte, error) {
	return json.Marshal(snap)
}
