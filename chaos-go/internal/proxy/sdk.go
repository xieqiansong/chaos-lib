package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"

	"chaos-go/config"

	"github.com/gin-gonic/gin"
)

const (
	linkName    = "current"
	versionFile = ".current-version"
)

// SdkSourceItem 是 Sources JSON 数组中的一个来源元素。
type SdkSourceItem struct {
	Kind string // "repo" | "single"
	Root string // 绝对路径
}

// SdkSource 一行代表一个 SDK 类型（jdk/maven/python/...）。
// Sources 为该类型下的来源数组（repo 与 single 可混合）；
// Current 为当前启用版本的绝对路径（单值即保证同时仅一个版本启用）。
type SdkSource struct {
	ID        uint            `gorm:"primaryKey"`
	Name      string          `gorm:"uniqueIndex:idx_sdk_sources_name,priority:1"` // SDK 类型，如 jdk / maven
	Sources   json.RawMessage `gorm:"type:jsonb"`                                    // [{"kind":"repo","root":"..."},{"kind":"single","root":"..."}]
	Current   string          // 当前启用版本绝对路径
	Enabled   bool   `gorm:"default:true"`
	Note      string
	IsDeleted bool `gorm:"default:false"`
}

// SdkInfo 返回给前端的版本信息。
type SdkInfo struct {
	CurrentVersion string
	VersionList    []string
}

// 默认种子：每个 SDK 类型含一个 repo 来源（兼容旧 D:\opt\xxx 布局）。
var defaultSdkSeeds = []struct {
	Name string
	Root string
}{
	{"jdk", `D:\opt\jdk`},
	{"maven", `D:\opt\maven`},
	{"python", `D:\opt\python`},
	{"llama", `D:\opt\llama`},
}

func init() {
	seedSdkSources()
}

// seedSdkSources 在 DB 为空时插入默认 SDK 类型，已存在则跳过。
func seedSdkSources() {
	db := config.GetDB()
	if db == nil {
		return
	}
	for _, s := range defaultSdkSeeds {
		var count int64
		db.Model(&SdkSource{}).Where("name = ? AND is_deleted = ?", s.Name, false).Count(&count)
		if count > 0 {
			continue
		}
		items := []SdkSourceItem{{Kind: "repo", Root: s.Root}}
		b, _ := json.Marshal(items)
		db.Create(&SdkSource{
			Name:    s.Name,
			Sources: b,
			Enabled: true,
		})
	}
}

// parseSources 解析 Sources JSON 数组。
func parseSources(raw json.RawMessage) []SdkSourceItem {
	var items []SdkSourceItem
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &items)
	}
	return items
}

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

// GetSdkVersions 返回所有启用 SDK 类型的版本信息，map key = 类型 Name。
func GetSdkVersions(c *gin.Context) {
	var srcs []SdkSource
	config.GetDB().Where("is_deleted = ?", false).Find(&srcs)
	result := make(map[string]SdkInfo)
	for _, s := range srcs {
		if !s.Enabled {
			continue
		}
		result[s.Name] = getSdkInfo(s)
	}
	c.JSON(200, result)
}

// GetSdkVersion 返回指定类型的版本信息。
func GetSdkVersion(c *gin.Context) {
	typ := c.Param("type")
	var s SdkSource
	if err := config.GetDB().Where("name = ? AND is_deleted = ?", typ, false).First(&s).Error; err != nil {
		c.JSON(404, gin.H{"error": "SDK type not found"})
		return
	}
	c.JSON(200, getSdkInfo(s))
}

// UpdateSdkVersion 切换版本：仅对 repo 来源生效，更新 symlink + .current-version + Current 绝对路径。
func UpdateSdkVersion(c *gin.Context) {
	typ := c.Param("type")
	db := config.GetDB()
	var s SdkSource
	if err := db.Where("name = ? AND is_deleted = ?", typ, false).First(&s).Error; err != nil {
		c.JSON(404, gin.H{"error": "SDK type not found"})
		return
	}

	var req struct {
		Version string
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON body"})
		return
	}

	// 仅对 repo 来源定位目标
	var targetRepo SdkSourceItem
	found := false
	for _, it := range parseSources(s.Sources) {
		if it.Kind != "repo" {
			continue
		}
		target := filepath.Join(it.Root, req.Version)
		if _, err := os.Stat(target); err == nil {
			targetRepo = it
			found = true
			break
		}
	}
	if !found {
		c.JSON(400, gin.H{"error": "Target version does not exist: " + req.Version})
		return
	}

	target := filepath.Join(targetRepo.Root, req.Version)
	link := filepath.Join(targetRepo.Root, linkName)
	verFile := filepath.Join(targetRepo.Root, versionFile)

	if _, err := os.Lstat(link); err == nil {
		os.Remove(link)
	}
	if err := os.Symlink(target, link); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create symlink: " + err.Error() + ". (Maybe need admin rights?)"})
		return
	}
	if err := os.WriteFile(verFile, []byte(req.Version), 0644); err != nil {
		c.JSON(500, gin.H{"error": "Failed to write version file: " + err.Error()})
		return
	}

	// 持久化当前版本绝对路径
	absCurrent := target
	db.Model(&s).Update("current", absCurrent)

	c.JSON(200, gin.H{
		"message": "Successfully switched to " + req.Version,
		"version": req.Version,
	})
}

// ---------------- SDK 类型 CRUD ----------------

// ListSdkSources 列出所有未删除的 SDK 类型。
func ListSdkSources(c *gin.Context) {
	var srcs []SdkSource
	config.GetDB().Where("is_deleted = ?", false).Find(&srcs)
	c.JSON(200, srcs)
}

// CreateSdkSource 新增 SDK 类型。
func CreateSdkSource(c *gin.Context) {
	var req SdkSource
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(400, gin.H{"error": "Name is required"})
		return
	}
	for _, it := range parseSources(req.Sources) {
		if it.Kind != "repo" && it.Kind != "single" {
			c.JSON(400, gin.H{"error": "Source kind must be 'repo' or 'single'"})
			return
		}
		if it.Root == "" {
			c.JSON(400, gin.H{"error": "Source root is required"})
			return
		}
	}
	var count int64
	config.GetDB().Model(&SdkSource{}).Where("name = ? AND is_deleted = ?", req.Name, false).Count(&count)
	if count > 0 {
		c.JSON(409, gin.H{"error": "SDK type already exists: " + req.Name})
		return
	}
	if err := config.GetDB().Create(&req).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, req)
}

// UpdateSdkSource 编辑 SDK 类型（Sources/Current/Enabled/Note）。
func UpdateSdkSource(c *gin.Context) {
	name := c.Param("name")
	db := config.GetDB()
	var s SdkSource
	if err := db.Where("name = ? AND is_deleted = ?", name, false).First(&s).Error; err != nil {
		c.JSON(404, gin.H{"error": "SDK type not found"})
		return
	}
	var patch struct {
		Sources json.RawMessage
		Current string
		Enabled *bool
		Note    string
	}
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{}
	if patch.Sources != nil {
		for _, it := range parseSources(patch.Sources) {
			if it.Kind != "repo" && it.Kind != "single" {
				c.JSON(400, gin.H{"error": "Source kind must be 'repo' or 'single'"})
				return
			}
			if it.Root == "" {
				c.JSON(400, gin.H{"error": "Source root is required"})
				return
			}
		}
		updates["sources"] = patch.Sources
	}
	if patch.Current != "" {
		updates["current"] = patch.Current
	}
	if patch.Enabled != nil {
		updates["enabled"] = *patch.Enabled
	}
	if patch.Note != "" || len(patch.Sources) > 0 {
		updates["note"] = patch.Note
	}
	if len(updates) == 0 {
		c.JSON(200, s)
		return
	}
	db.Model(&s).Updates(updates)
	c.JSON(200, s)
}

// DeleteSdkSource 软删除 SDK 类型。
func DeleteSdkSource(c *gin.Context) {
	name := c.Param("name")
	db := config.GetDB()
	var s SdkSource
	if err := db.Where("name = ? AND is_deleted = ?", name, false).First(&s).Error; err != nil {
		c.JSON(404, gin.H{"error": "SDK type not found"})
		return
	}
	db.Model(&s).Update("is_deleted", true)
	c.JSON(200, gin.H{"message": "deleted: " + name})
}
