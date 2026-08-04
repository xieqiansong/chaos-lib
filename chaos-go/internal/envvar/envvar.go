package envvar

import (
	"encoding/json"
	"fmt"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// ── 模型 ──────────────────────────────────────────────────────────

type EnvScope string

const (
	EnvScopeSystem EnvScope = "system"
	EnvScopeUser   EnvScope = "user"
)

type EnvMeta struct {
	SavedAt  string `toml:"saved_at"`
	Hostname string `toml:"hostname,omitempty"`
	Username string `toml:"username,omitempty"`
}

type EnvSection map[string]string

type EnvSnapshot struct {
	Meta   EnvMeta    `toml:"meta"`
	System EnvSection `toml:"system"`
	User   EnvSection `toml:"user"`
}

type EnvPathPatch struct {
	Prepend []string
	Append  []string
	Remove  []string
	Replace []string
}

type EnvSectionPatch struct {
	Set   map[string]string
	Unset []string
	Path  EnvPathPatch
}

type EnvPatchRequest struct {
	System *EnvSectionPatch
	User   *EnvSectionPatch
}

type EnvPutRequest struct {
	System *EnvSection
	User   *EnvSection
}

type EnvGetResponse struct {
	Meta         EnvMeta
	System       EnvSection
	User         EnvSection
	SnapshotID   int
	SnapshotTime string
	Warnings     []string
}

type EnvApplyResponse struct {
	Message      string
	SnapshotID   int
	SnapshotTime string
	Warnings     []string
}

// ── TOML 序列化 ──────────────────────────────────────────────────

func MarshalEnvToTOML(snapshot *EnvSnapshot) (string, error) {
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

// ── 字符串工具 ──────────────────────────────────────────────────

func splitAndTrim(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	parts := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if len(sep) > 0 && i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			part := trimSpaces(s[start:i])
			if part != "" {
				parts = append(parts, part)
			}
			i += len(sep) - 1
			start = i + 1
		}
	}
	if start < len(s) {
		part := trimSpaces(s[start:])
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func trimSpaces(s string) string {
	left := 0
	right := len(s)
	for left < right && (s[left] == ' ' || s[left] == '\t' || s[left] == '\r' || s[left] == '\n') {
		left++
	}
	for right > left && (s[right-1] == ' ' || s[right-1] == '\t' || s[right-1] == '\r' || s[right-1] == '\n') {
		right--
	}
	return s[left:right]
}

func joinNonEmpty(items []string, sep string) string {
	nonEmpty := make([]string, 0, len(items))
	for _, item := range items {
		if item != "" {
			nonEmpty = append(nonEmpty, item)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	result := nonEmpty[0]
	for i := 1; i < len(nonEmpty); i++ {
		result += sep + nonEmpty[i]
	}
	return result
}

func dedupedCopy(items []string) []string {
	if items == nil {
		return []string{}
	}
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func cloneMap(m map[string]string) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// ── JSON 辅助（避免循环引用 reflect） ─────────────────────────────

func EnvSnapshotToJSON(snap *EnvSnapshot) ([]byte, error) {
	return json.Marshal(snap)
}
