package sdk

import (
	"encoding/json"
	"log/slog"

	"chaos-go/internal/config"
)

// 默认种子：库内无 SDK 类型时写入（兼容旧 D:\opt\xxx 布局）。
var defaultSdkSeeds = []struct {
	Name string
	Root string
}{
	{"jdk", `D:\opt\jdk`},
	{"maven", `D:\opt\maven`},
	{"python", `D:\opt\python`},
	{"llama", `D:\opt\llama`},
}

// SeedDefaults 在库内无 SDK 类型时写入默认类型，已存在则跳过。
// 由启动流程显式调用（不在 init 里做副作用）。
func SeedDefaults() {
	db := config.GetDB()
	if db == nil {
		slog.Warn("SDK 默认类型播种跳过", "reason", "数据库不可用")
		return
	}
	seeded := 0
	for _, s := range defaultSdkSeeds {
		var count int64
		if err := db.Model(&SdkSource{}).Where("name = ? AND is_deleted = ?", s.Name, false).Count(&count).Error; err != nil {
			slog.Error("统计 SDK 类型失败", "name", s.Name, "err", err)
			continue
		}
		if count > 0 {
			continue
		}
		items := []SdkSourceItem{{Kind: "repo", Root: s.Root}}
		b, _ := json.Marshal(items)
		if err := db.Create(&SdkSource{Name: s.Name, Sources: b, Enabled: true}).Error; err != nil {
			slog.Error("写入默认 SDK 类型失败", "name", s.Name, "err", err)
			continue
		}
		seeded++
	}
	if seeded > 0 {
		slog.Info("已写入默认 SDK 类型", "count", seeded)
	}
}
