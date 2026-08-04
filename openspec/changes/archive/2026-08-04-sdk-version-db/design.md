# Design: SDK 来源入库（按 SDK 类型组织 + JSON 来源数组）

## 决策

1. **一行 = 一个 SDK 类型**：`SdkSource` 表不再「一行 = 一个来源」，而是「一行 = 一个 SDK 类型（jdk/maven/python/...）」。每个类型下可有多个来源（repo + single 混合），来源集合以 `Sources` 的 JSON 数组表达。
2. **两种来源形态，存进 JSON**：`Sources` 数组元素为 `{ "kind": "repo"|"single", "root": "<绝对路径>" }`。
   - `repo`：父目录，子目录 = 版本。`root` 如 `D:\opt\jdk`。
   - `single`：单目录即一个版本。`root` 即版本目录，版本号取 `filepath.Base(root)`。
3. **`Current` 存绝对路径**：当前启用版本的绝对路径（repo 为 `root/<子目录名>`，single 为其 `root`）。单值即天然保证「同一时刻只有一个版本启用」，避免给每个子版本逐一标 enable 的混乱；切换时同时改 `Current` 与重建 `current` symlink，两者永远一致。
4. **配置与机制分离**：来源「是什么 / 在哪 / 是否启用」入 DB；「怎么切换」保留在文件系统层（仅 repo 涉及 symlink + 版本文件）。
5. **版本列表策略**：
   - `repo`：返回版本时实时扫描 `root` 直接子目录（排除 `current`、隐藏文件如 `.current-version`），属运行时事实，不入库；版本绝对路径 `== Current` 标为启用。
   - `single`：`VersionList = [filepath.Base(root)]`，`Current == root` 即启用，无磁盘扫描。
6. **删除 `services/sdk.go` 死代码**：grep 确认 `services.GetSdk*` 零引用，直接删除。
7. **种子数据**：`internal/proxy` 包 `init()` 中读取 DB，若不存在 `jdk/maven/python/llama` 记录则插入默认 4 条（各含一个 `kind=repo`、`root=D:\opt\xxx` 的来源，`Current` 取 `root` 下首个子目录或留空待首次切换），兼容现有 `D:\opt\xxx` 布局。
8. **软删除遵循项目规则**：`IsDeleted bool`，查询手动加 `WHERE is_deleted = false`。
9. **`Name` 唯一**：作为 `GET /sdks` 返回 map 的 key 与切换接口的 `:name` 参数；唯一索引防重。

## 模型

```go
package proxy

import "github.com/google/uuid" // 仅示意；实际用 datatypes.JSON

type SdkSource struct {
	ID       uint           `gorm:"primaryKey"`
	Name     string         `gorm:"uniqueIndex:idx_sdk_sources_name,priority:1"` // SDK 类型，如 jdk / maven
	Sources  datatypes.JSON `gorm:"type:jsonb"` // [{"kind":"repo","root":"D:\\opt\\jdk"},{"kind":"single","root":"C:\\users\\x\\jdk11"}]
	Current  string         // 当前启用版本绝对路径
	Enabled  bool           `gorm:"default:true"`
	Note     string
	IsDeleted bool          `gorm:"default:false"`
}
```

## 实现要点

- sources 元素结构：

  ```go
  type SdkSourceItem struct {
      Kind string `json:"kind"` // "repo" | "single"
      Root string `json:"root"` // 绝对路径
  }
  ```

- 列表接口改为：

  ```go
  func GetSdkVersions(c *gin.Context) {
      var srcs []SdkSource
      config.GetDB().Where("is_deleted = ?", false).Find(&srcs)
      result := make(map[string]SdkInfo)
      for _, s := range srcs {
          if !s.Enabled { continue }
          result[s.Name] = getSdkInfo(s)
      }
      c.JSON(200, result)
  }
  ```

- 单类型版本推导：

  ```go
  func getSdkInfo(s SdkSource) SdkInfo {
      var items []SdkSourceItem
      json.Unmarshal(s.Sources, &items)
      var info SdkInfo
      for _, it := range items {
          if it.Kind == "single" {
              abs := it.Root
              info.VersionList = append(info.VersionList, filepath.Base(abs))
              if abs == s.Current { info.CurrentVersion = filepath.Base(abs) }
              continue
          }
          // repo：扫描 root 子目录
          entries, _ := os.ReadDir(it.Root)
          for _, e := range entries {
              if !e.IsDir() { continue }
              if e.Name() == linkName || e.Name() == versionFile { continue }
              abs := filepath.Join(it.Root, e.Name())
              info.VersionList = append(info.VersionList, e.Name())
              if abs == s.Current { info.CurrentVersion = e.Name() }
          }
      }
      return info
  }
  ```

- 切换接口 `UpdateSdkVersion`：
  - 用 `Name` 从 DB 取 `SdkSource`；找不到返回 404。
  - 遍历 `Sources`，仅对 `Kind=repo` 的 item 用 `filepath.Join(it.Root, version)` 定位目标；single 项不参与切换。
  - 命中 repo 项后：重建 `root/current` symlink + 写 `root/.current-version`，并把该类型的 `Current` 字段更新为 `filepath.Join(it.Root, version)` 绝对路径，持久化。
  - 若请求的 version 不在任何 repo 子目录下 → 400。

- 新增 CRUD（`SDK 类型` 管理）：
  - `GET /api/sdks/defs` 列出全部 SDK 类型（含各自 sources 数组）
  - `POST /api/sdks/defs` 新增（body: Name, Sources[], Current?, Enabled?, Note?）
  - `PATCH /api/sdks/defs/:name` 编辑（Sources/Current/Enabled/Note）
  - `DELETE /api/sdks/defs/:name` 软删除

## 风险

- seed 与手动数据冲突：以 `uniqueIndex` 的 `Name` 去重，seed 用 `OnConflict` 跳过已存在，不覆盖用户改动。
- `Root` 路径在部署机上可能不同：种子默认 Windows `D:\opt\xxx`，其他环境用户自行改 Root，符合「配置即数据」初衷。
- `single` 模式无切换能力，前端若对 single 显示切换按钮应禁用——属前端增量，本次不强制。
- `Current` 绝对路径依赖具体机器目录布局，跨机迁移时需重新配置——属预期。
