package note

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"chaos-go/internal/framework/config"
	"gorm.io/gorm"
)

// db 返回数据库单例；本包唯一的 DB 出口（config.GetDB 只允许出现在 repository 层）。
func db() *gorm.DB { return config.GetDB() }

// ListFilter 列表筛选参数。dir 为空表示根目录（直属于 vault 根的文件）。
type ListFilter struct {
	Dir      string
	Query    string
	Starred  bool
	Missing  bool
	Offset   int
	PageSize int
}

// FindByID 按 id 取（软删过滤由 soft_delete 插件处理）。
func FindByID(id int) (*Note, error) {
	var n Note
	if err := db().First(&n, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}
	return &n, nil
}

// queryNotes 按筛选条件查询笔记（含分页）。软删过滤由 soft_delete 插件自动追加。
// 全文检索用 LOWER(col) LIKE LOWER(?) 以兼容 PostgreSQL 与 SQLite 且大小写不敏感。
func queryNotes(filter ListFilter) ([]Note, int64, error) {
	base := db().Model(&Note{})
	if filter.Dir != "" {
		base = base.Where("parent_rel = ?", filter.Dir)
	} else {
		base = base.Where("parent_rel = ?", "")
	}
	if filter.Query != "" {
		pat := "%" + strings.ToLower(filter.Query) + "%"
		base = base.Where("LOWER(title) LIKE ? OR LOWER(summary) LIKE ? OR LOWER(search_text) LIKE ?", pat, pat, pat)
	}
	if filter.Starred {
		base = base.Where("starred = ?", true)
	}
	if filter.Missing {
		base = base.Where("disk_missing = ?", true)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var notes []Note
	if err := base.Order("id DESC").Offset(filter.Offset).Limit(filter.PageSize).Find(&notes).Error; err != nil {
		return nil, 0, err
	}
	return notes, total, nil
}

// loadAllNotes 加载全部未删索引项（供目录树与扫描增量比对）。
func loadAllNotes() ([]Note, error) {
	var notes []Note
	if err := db().Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

// saveNote 插入或更新一条索引（Save 按主键存在与否决定 INSERT/UPDATE）。
func saveNote(n *Note) error {
	return db().Save(n).Error
}

// markMissing 批量设置 disk_missing 标记（扫描后：在盘缺失项置 true，已复位项置 false）。
func markMissing(rels []string, missing bool) error {
	return db().Model(&Note{}).Where("rel_path IN ?", rels).Update("disk_missing", missing).Error
}

// relocateIndex 级联重写路径前缀（支持目录整体移动）：把匹配 oldRel 及其后代的所有索引项，
// 把 rel_path 前缀替换为 newRel，并重算派生字段（标题 / 摘要 / hash 等来自磁盘新位置）。
// 文件已在磁盘层完成移动，此处只更新数据库。
func relocateIndex(oldRel, newRel string) error {
	var affected []Note
	if err := db().Where("rel_path = ? OR rel_path LIKE ?", oldRel, oldRel+"/%").Find(&affected).Error; err != nil {
		return err
	}
	for i := range affected {
		o := affected[i].RelPath
		var nu string
		if o == oldRel {
			nu = newRel
		} else {
			nu = newRel + o[len(oldRel):]
		}
		abs, err := absPath(nu)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return err
		}
		updated := parseNoteContent(nu, filepath.Base(nu), content, info.ModTime(), int(info.Size()))
		updated.ID = affected[i].ID
		updated.VaultID = affected[i].VaultID
		updated.Starred = affected[i].Starred
		if err := saveNote(&updated); err != nil {
			return err
		}
	}
	return nil
}

// deleteIndex 硬删一条索引行（回收站场景下文件已保留在 .trash，行可移除以便重扫重建）。
func deleteIndex(id int) error {
	return db().Unscoped().Where("id = ?", id).Delete(&Note{}).Error
}
