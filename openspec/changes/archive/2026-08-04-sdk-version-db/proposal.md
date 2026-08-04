# Change Proposal: SDK 版本管理数据入库（JSON 来源数组，按 SDK 类型组织）

## Why

当前 SDK 类型与根路径（`jdk`/`maven`/`python`/`llama` → `D:\opt\xxx`）以 `map[string]string` 形式**写死在两份重复代码**里：

- `internal/proxy/sdk.go`（路由实际引用）
- `services/sdk.go`（死代码，无人引用，与前者完全重复）

这导致：

- 新增/删除/改名 SDK 必须改 Go 代码并重新编译部署，无法运行时调整；
- 同一逻辑两份拷贝，极易出现不一致；
- **只能表达「一个类型 = 一个仓库目录」一种形态**：仓库下子目录即版本。无法表达「一个目录就是一个独立版本」，也无法配置多个仓库 / 多个零散版本。

目标：把 SDK 来源配置化、可自由组合。支持两类来源：

1. **仓库（repo）**：一个父目录下，每个子文件夹是一个版本号（沿用现有扫描 + `current` 软链接机制）。
2. **单个版本（single）**：一个目录本身即一个版本，由目录名标识（无子目录扫描，无版本切换）。

两类来源都入库，可任意添加多个，切换版本的底层机制（仓库用 symlink + `.current-version`；single 仅记录当前版本）保持不变。

## What Changes

- 新增 GORM 模型 `SdkSource`（SDK 来源表），**一行代表一个 SDK 类型**（如 `jdk`/`maven`/`python`/`npm`），字段：
  - `Name`：SDK 类型唯一标识，作 map key 与切换 `:name`。
  - `Sources`（`jsonb`）：该类型下所有来源的 JSON 数组，每个元素 `{ "kind": "repo"|"single", "root": "<绝对路径>" }`，repo 与 single 可混合共存。
  - `Current`：当前启用版本的**绝对路径**（repo 下为 `root/<子目录名>`，single 下即其 `root`）；同一时刻唯一，天然保证「只有一个版本启用」。
  - `Enabled`、`Note`、`IsDeleted`。
- 版本列表策略：
  - `repo`：返回版本时**实时扫描 `root` 子目录**得到版本号列表（取目录名，排除 `current` 与 `.current-version`），不入库；其中绝对路径 `== Current` 的标为启用。
  - `single`：版本号即 `root` 目录名，绝对路径 `== Current` 即启用，无扫描、无切换。
- `internal/proxy/sdk.go` 改为从 DB 读取 `SdkSource`，而非 `sdkRoots` 字面量 map。
- 删除死代码 `services/sdk.go`。
- 新增 CRUD 接口管理 SDK 类型与来源数组（新增/编辑/删除/列出），切换版本接口 `PATCH /api/sdks/:name/switch` 仅对 `repo` 生效（single 返回 400）。
- 在 `cmd/server/main.go` 的 `AutoMigrate()` 登记 `SdkSource`；首次启动以现有 4 个仓库做种子数据（seed）。

## Impact

- 受影响文件：`internal/proxy/sdk.go`、`services/sdk.go`（删除）、`cmd/server/main.go`、`routes/routes.go`（新增 CRUD 路由）。
- 现有前端 `chaos-ui/src/views/Sdk.vue` 的读取（`GET /sdks`，map 的 key 从原 `Type` 变为 SDK 类型 `Name`）与切换行为**契约基本不变**；本变更聚焦后端与 API，前端管理 UI 为后续增量（不在本次强制范围）。
- 数据迁移：无历史表，首次启动 seed 4 条 SDK 类型记录（各含一个 repo 来源）；若用户此前手动改过 `sdkRoots`，需重新通过接口配置（属预期，配置即数据）。
