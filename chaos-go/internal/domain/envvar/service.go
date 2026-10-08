package envvar

import (
	"fmt"
	"time"

	"chaos-go/pkg/tools"

	"github.com/pelletier/go-toml/v2"
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

// commitSnapshot 把快照序列化为 TOML 并落一条快照，返回过程中的非致命告警。
// 序列化失败时返回 error（阻断调用方）；落快照失败仅作为 warning 累加。
func commitSnapshot(fileID int, snap *EnvSnapshot) ([]string, error) {
	content, err := MarshalEnvToTOML(snap)
	if err != nil {
		return nil, err
	}
	if _, snapErr := takeSnapshot(fileID, content); snapErr != nil {
		return []string{fmt.Sprintf("快照失败: %v", snapErr)}, nil
	}
	return nil, nil
}

// Load 读取当前系统环境变量，并附最近一次快照的 id 与时间。
// scope 为空时返回 system 与 user 两段；为 EnvScopeSystem / EnvScopeUser 时仅读取并返回对应段，
// 便于 UI 按「用户 / 系统」分区独立加载与刷新（未请求的段不再读取，也避免无谓的权限告警）。
func Load(scope EnvScope) (*EnvGetResponse, error) {
	if scope != "" && scope != EnvScopeSystem && scope != EnvScopeUser {
		return nil, fmt.Errorf("非法的作用域: %s，仅支持 system / user", string(scope))
	}
	fileID, err := EnsureVirtualFile()
	if err != nil {
		return nil, err
	}
	resp := &EnvGetResponse{
		System:   map[string]string{},
		User:     map[string]string{},
		Warnings: []string{},
	}
	if scope == "" || scope == EnvScopeSystem {
		if vars, e := readScopeViaTools(EnvScopeSystem); e != nil {
			resp.Warnings = append(resp.Warnings, fmt.Sprintf("读取系统变量失败: %v", e))
			resp.System = map[string]string{}
		} else {
			resp.System = vars
		}
	}
	if scope == "" || scope == EnvScopeUser {
		if vars, e := readScopeViaTools(EnvScopeUser); e != nil {
			resp.Warnings = append(resp.Warnings, fmt.Sprintf("读取用户变量失败: %v", e))
			resp.User = map[string]string{}
		} else {
			resp.User = vars
		}
	}
	resp.Meta = EnvMeta{Hostname: tools.GetHostname(), Username: tools.GetUsername()}
	if latest, e := latestSnapshot(fileID); e == nil && latest != nil {
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
	warnings, commitErr := commitSnapshot(fileID, snap)
	if commitErr != nil {
		return nil, commitErr
	}
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

	snapWarnings, commitErr := commitSnapshot(fileID, snap)
	if commitErr != nil {
		return warnings, commitErr
	}
	warnings = append(warnings, snapWarnings...)
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
	snapWarnings, commitErr := commitSnapshot(fileID, current)
	if commitErr != nil {
		return warnings, commitErr
	}
	warnings = append(warnings, snapWarnings...)
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

// ── Windows 系统环境变量读写（经 tools 包原语）─────────────────
//
// 作用域映射与落盘编排。低层 win32 原语（提权判断、计算机名 / 用户名、
// 广播 WM_SETTINGCHANGE）由 pkg/tools/env_var_windows.go 提供，跨平台编译时
// 由 pkg/tools/env_var_other.go 给出占位实现。

// domainScopeToTools 将领域层作用域（system/user）映射为 tools 包的作用域（Machine/User）。
// 领域层的 system 对应用户变量注册表项 HKLM\...\Environment，即 .NET 的 Machine 作用域；
// user 对应 HKCU\Environment，即 .NET 的 User 作用域。
func domainScopeToTools(scope EnvScope) (tools.EnvScope, bool) {
	switch scope {
	case EnvScopeSystem:
		return tools.EnvScopeMachine, true
	case EnvScopeUser:
		return tools.EnvScopeUser, true
	default:
		return "", false
	}
}

// readScopeViaTools 通过 tools 包读取某一作用域下的全部环境变量，转为 map。
func readScopeViaTools(scope EnvScope) (map[string]string, error) {
	ts, ok := domainScopeToTools(scope)
	if !ok {
		return nil, fmt.Errorf("未知作用域: %s", string(scope))
	}
	vars, err := tools.ListEnvVars(ts)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(vars))
	for _, v := range vars {
		out[v.Name] = v.Value
	}
	return out, nil
}

// applyScope 将期望变量集合落盘到指定作用域：仅对变化项做 Set，对删除项做 Remove。
// 返回本次作用域内的警告（非致命错误）列表。
func applyScope(scope EnvScope, desired, current map[string]string) []string {
	ts, ok := domainScopeToTools(scope)
	if !ok {
		return []string{fmt.Sprintf("未知作用域: %s", string(scope))}
	}
	var warnings []string
	sets, removes := 0, 0
	for k, v := range desired {
		if oldVal, existed := current[k]; existed && oldVal == v {
			continue
		}
		sets++
		if e := tools.SetEnvVar(k, v, ts); e != nil {
			warnings = append(warnings, fmt.Sprintf("设置 %s 失败: %v", k, e))
		}
	}
	for k := range current {
		if _, ok := desired[k]; !ok {
			removes++
			if e := tools.RemoveEnvVar(k, ts); e != nil {
				warnings = append(warnings, fmt.Sprintf("删除 %s 失败: %v", k, e))
			}
		}
	}
	return warnings
}

// ReadAllEnvFromSystem 读取系统 / 用户两级环境变量，组成快照。
func ReadAllEnvFromSystem() (*EnvSnapshot, error) {
	sysVars, errSys := readScopeViaTools(EnvScopeSystem)
	if errSys != nil {
		sysVars = map[string]string{}
	}
	userVars, errUser := readScopeViaTools(EnvScopeUser)
	if errUser != nil {
		userVars = map[string]string{}
	}
	host := tools.GetHostname()
	user := tools.GetUsername()
	snap := &EnvSnapshot{
		Meta:   EnvMeta{Hostname: host, Username: user},
		System: sysVars,
		User:   userVars,
	}
	if snap.System == nil {
		snap.System = map[string]string{}
	}
	if snap.User == nil {
		snap.User = map[string]string{}
	}
	if errSys != nil || errUser != nil {
		var combined error
		if errSys != nil {
			combined = errSys
		} else {
			combined = errUser
		}
		return snap, combined
	}
	return snap, nil
}

// WriteAllEnvToSystem 将快照落盘：系统级需管理员；用户级始终尝试；最后广播环境变更。
func WriteAllEnvToSystem(snap *EnvSnapshot) (warnings []string, err error) {
	warnings = []string{}
	originalSystem, _ := readScopeViaTools(EnvScopeSystem)
	if originalSystem == nil {
		originalSystem = map[string]string{}
	}
	originalUser, _ := readScopeViaTools(EnvScopeUser)
	if originalUser == nil {
		originalUser = map[string]string{}
	}
	elevated := tools.IsElevated()
	if elevated {
		if w := applyScope(EnvScopeSystem, cloneMap(snap.System), originalSystem); len(w) > 0 {
			warnings = append(warnings, fmt.Sprintf("系统级变量部分写入失败: %v", w))
		}
	} else {
		warnings = append(warnings, "未以管理员身份运行，系统级变量未写入")
	}
	if w := applyScope(EnvScopeUser, cloneMap(snap.User), originalUser); len(w) > 0 {
		warnings = append(warnings, fmt.Sprintf("用户级变量写入失败: %v", w))
	}
	return warnings, nil
}
