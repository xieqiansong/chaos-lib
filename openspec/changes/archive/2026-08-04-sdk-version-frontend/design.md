# Design: SDK 版本管理前端

## 数据契约（复用后端）

- `GET /api/sdks` → `Record<typeName, { CurrentVersion: string, VersionList: string[] }>`（只读版本视图，行为不变）。
- `GET /api/sdks/defs` → `SdkSource[]`，每条：
  ```ts
  interface SdkSource {
    ID: number
    Name: string
    Sources: { kind: 'repo' | 'single'; root: string }[]
    Current: string   // 当前启用版本绝对路径
    Enabled: boolean
    Note: string
    IsDeleted: boolean
  }
  ```
- `POST /api/sdks/defs` body：`{ Name, Sources, Enabled?, Note? }`。
- `PATCH /api/sdks/defs/:name` body：`{ Sources?, Current?, Enabled?, Note? }`。
- `DELETE /api/sdks/defs/:name` → 软删除。
- `PATCH /api/sdks/:type/switch` body：`{ version }`，仅对 repo 生效（single 后端返回 400）。

## 前端结构设计

### 1. 来源可视化
- `Sdk.vue` 新增 `defs` 状态，挂载时并行拉 `GET /sdks` 与 `GET /sdks/defs`。
- 每个 SDK 类型行展示：
  - 类型名（来自 defs.Name，作为 key）。
  - 来源列表：`sources` 中每项渲染为 `repo`/`single` 徽标 + `root` 路径文本。
  - 版本标签区：来自 `GET /sdks` 的 `VersionList`，高亮 `CurrentVersion`；点击切换（仅 repo 来源提供的版本可点）。

### 2. 切换语义修正
- 维护「哪些版本可切换」集合：遍历该类型的 `sources`，仅 `repo` 来源的 root 下子目录名可切换；single 来源的目录名不可切换。
- 简化实现：由于 `GET /sdks` 合并了所有来源版本，无法区分来源类型。因此**可切换判定**改为：在 defs 中若类型**仅含 single 来源**，则整个类型禁用切换；若含 ≥1 个 repo 来源，则其版本来自 repo（当前数据模型下 repo 子目录即版本），允许切换。single 与 repo 混合时，版本列表为并集，前端无法精确区分某版本属哪来源——采用**保守策略**：含 repo 即允许切换（匹配后端「仅 repo 定位」逻辑，single 同名也不会误切，因为 switch 接口只在 repo root 下查找）。
- single 单独成类型时：版本标签置灰、点击无效并提示「单版本来源不可切换」。

### 3. 管理界面
- 在 Sdk.vue 增加「管理来源」抽屉/对话框（`el-dialog`）：
  - 列表展示所有 `defs`（含 `Enabled=false`）。
  - 新增：`Name` 输入 + `sources` 动态表单（每项 kind 下拉 + root 路径输入 + 增删按钮）+ `Enabled` 开关 + `Note`。
  - 编辑：加载该类型 def，可改 `sources`/`Enabled`/`Note`。
  - 删除：确认后 `DELETE`。
- 操作后刷新 `defs` 与 `sdks`。

## 错误处理
- defs 接口失败不影响版本展示（降级为只读）。
- switch 返回 400（single 误切）时 `ElMessage` 提示「该来源不支持切换」。

## 风险
- 前端无法从 `GET /sdks` 区分版本来自 repo 还是 single；采用上"含 repo 即允许切换"的保守策略，与后端 switch 查找逻辑一致。若未来需要精确区分，后端应在版本对象里带 `kind` 字段（后续增量，不在本次）。
- single 类型的 root 路径依赖具体机器，管理界面输入需用户自行保证正确。
