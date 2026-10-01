package project

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"chaos-go/pkg/tools"
)

// ── 写方向钩子（由 crud.Opts 回调转调，见 handler.go 的 Register）──

// ValidateGroupForCreate 建组前校验名称 / 根目录，并创建根目录。
func ValidateGroupForCreate(g *ProjectGroup) error {
	if g.Name == "" {
		return fmt.Errorf("项目组名称不能为空")
	}
	if g.AbsolutePath == "" {
		return fmt.Errorf("根目录绝对路径不能为空")
	}
	if err := os.MkdirAll(g.AbsolutePath, 0o755); err != nil {
		return fmt.Errorf("根目录不存在且创建失败: %s", err.Error())
	}
	g.AbsolutePath = filepath.Clean(g.AbsolutePath)
	return nil
}

// RelocateProjectsAfterGroupUpdate 组根目录变更后重算各子项目的绝对路径。
func RelocateProjectsAfterGroupUpdate(g *ProjectGroup) error {
	children, err := FindActiveProjectsByGroup(g.ID)
	if err != nil {
		return err
	}
	for _, c := range children {
		newAbs := filepath.Join(g.AbsolutePath, c.RelativePath)
		if newAbs != c.AbsolutePath {
			if err := UpdateProjectAbsolutePath(c.ID, newAbs); err != nil {
				return err
			}
		}
	}
	return nil
}

// CascadeDeleteProjects 删组（软删）后级联软删其子项目。
func CascadeDeleteProjects(g *ProjectGroup) error {
	return CascadeDeleteProjectsByGroup(g.ID)
}

// PrepareProjectForCreate 建项目前解析路径、校验目录存在、补全名称与访问时间。
func PrepareProjectForCreate(p *Project) error {
	if p.GroupID == 0 {
		return fmt.Errorf("所属项目组ID不能为空")
	}
	group, err := FindGroupByID(p.GroupID)
	if err != nil {
		return fmt.Errorf("所属项目组不存在")
	}
	abs, rel, err := resolveProjectPaths(*group, p.AbsolutePath, p.RelativePath)
	if err != nil {
		return err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return fmt.Errorf("项目目录不存在: %s", abs)
	}
	p.AbsolutePath = abs
	p.RelativePath = rel
	if p.Name == "" {
		p.Name = filepath.Base(abs)
	}
	createdAt := tools.DirCreatedAt(abs)
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	p.CreatedAt = createdAt
	last := createdAt
	p.LastAccessedAt = &last
	return nil
}

// RemoveProjectDir 删项目（软删）后清空物理目录；
// 物理删除失败仅记录，DB 记录照常软删（与历史行为一致）。
func RemoveProjectDir(p *Project) error {
	if err := tools.RemoveDirSafe(p.AbsolutePath); err != nil {
		slog.Warn("项目物理目录删除失败（已软删记录）", "path", p.AbsolutePath, "err", err)
	}
	return nil
}

// ── 列表（派生：已认领 ∪ 磁盘未认领）────────────────────────────

// ListProjectItems 按 groupId / name 过滤项目；指定 groupId 时合并该组根目录下
// 尚未入库的子目录（Claimed=false）。
func ListProjectItems(groupID *int, name string) ([]ProjectListItem, error) {
	projects, err := ListActiveProjects(groupID, name)
	if err != nil {
		return nil, err
	}
	items := make([]ProjectListItem, 0, len(projects))
	if groupID == nil {
		for _, p := range projects {
			items = append(items, ProjectListItem{Project: p, Claimed: true})
		}
		return items, nil
	}
	group, err := FindGroupByID(*groupID)
	if err != nil {
		return nil, err
	}
	return mergeUnclaimed(*group, projects), nil
}

// mergeUnclaimed 把已认领项目与磁盘未认领子目录合并；
// 未认领项用负哨兵 ID，避免与已认领项（正 ID）冲突导致前端行 key 重复。
func mergeUnclaimed(group ProjectGroup, dbProjects []Project) []ProjectListItem {
	claimedSet := make(map[string]Project, len(dbProjects))
	for _, p := range dbProjects {
		claimedSet[filepath.Clean(p.AbsolutePath)] = p
	}

	items := make([]ProjectListItem, 0, len(dbProjects))
	for _, p := range dbProjects {
		items = append(items, ProjectListItem{Project: p, Claimed: true})
	}

	entries, err := os.ReadDir(group.AbsolutePath)
	if err != nil {
		return items
	}

	type unclaimed struct {
		item ProjectListItem
		name string
	}
	var unclaimedList []unclaimed
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		abs := filepath.Clean(filepath.Join(group.AbsolutePath, e.Name()))
		if _, ok := claimedSet[abs]; ok {
			continue
		}
		rel, relErr := filepath.Rel(group.AbsolutePath, abs)
		if relErr != nil {
			rel = e.Name()
		}
		item := ProjectListItem{
			Project: Project{
				GroupID:      group.ID,
				Name:         e.Name(),
				AbsolutePath: abs,
				RelativePath: rel,
			},
			Claimed: false,
		}
		item.ID = -(len(unclaimedList) + 1)
		unclaimedList = append(unclaimedList, unclaimed{item: item, name: e.Name()})
	}

	sort.Slice(unclaimedList, func(i, j int) bool {
		return unclaimedList[i].name < unclaimedList[j].name
	})
	for _, u := range unclaimedList {
		items = append(items, u.item)
	}
	return items
}

// ── 扩展动作 ────────────────────────────────────────────────────

// MoveProject 移动项目文件夹（同卷 rename / 跨卷 copy）并同步路径字段。
// 落库失败时撤掉已复制的新目录，避免磁盘与库不一致。
func MoveProject(id int, req MoveProjectRequest) error {
	project, err := FindProjectByID(id)
	if err != nil {
		return err
	}
	group, err := FindGroupByID(req.TargetGroupID)
	if err != nil {
		return err
	}
	newAbs, newRel, err := resolveProjectPaths(*group, req.TargetAbsPath, req.TargetRelativePath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}

	oldAbs := project.AbsolutePath
	moved := newAbs != oldAbs
	if moved {
		// tools.MoveProjectFolder 仅复制、不删源，需显式清理旧目录。
		if err := tools.MoveProjectFolder(oldAbs, newAbs); err != nil {
			return fmt.Errorf("移动文件夹失败: %w", err)
		}
		if err := tools.RemoveDirSafe(oldAbs); err != nil {
			slog.Warn("移动后清理旧目录失败（新目录已就位）", "path", oldAbs, "err", err)
		}
	}
	if err := UpdateProjectLocation(id, group.ID, newAbs, newRel); err != nil {
		if moved {
			_ = tools.RemoveDirSafe(newAbs)
		}
		return fmt.Errorf("更新路径失败: %w", err)
	}
	return nil
}

// RecordAccess 记录项目访问时间。
func RecordAccess(id int) error {
	if _, err := FindProjectByID(id); err != nil {
		return err
	}
	return TouchProjectAccess(id)
}
