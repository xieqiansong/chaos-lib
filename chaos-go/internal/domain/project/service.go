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

func CascadeDeleteProjects(g *ProjectGroup) error {
	return CascadeDeleteProjectsByGroup(g.ID)
}

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

func RemoveProjectDir(p *Project) error {
	if err := tools.RemoveDirSafe(p.AbsolutePath); err != nil {
		slog.Warn("项目物理目录删除失败（已软删记录）", "path", p.AbsolutePath, "err", err)
	}
	return nil
}

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

func RecordAccess(id int) error {
	if _, err := FindProjectByID(id); err != nil {
		return err
	}
	return TouchProjectAccess(id)
}
