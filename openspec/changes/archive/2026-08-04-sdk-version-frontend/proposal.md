# Change Proposal: SDK 版本管理前端（来源可视化 + 来源/类型管理）

## Why

后端已落地 `sdk-version-db` 变更：SDK 来源入库，每个 SDK 类型可含多个 `repo`/`single` 来源，并提供 `/sdks/defs` CRUD 接口。但现有前端 `chaos-ui/src/views/Sdk.vue` 仍停留在「只读 + 切换」阶段：

- 只调用 `GET /api/sdks`，完全不展示每个类型下的 `sources` 来源数组（repo/single 与 root 路径），用户无法看到配置；
- 不区分 repo 与 single，对 single 来源也提供切换点击，实际后端会返回 400；
- 没有管理入口：无法在前端新增/编辑/删除 SDK 类型或其来源数组，必须改后端 DB 才行。

目标：补齐前端，让来源配置**可见、可管理、可正确切换**。

## What Changes

- 改造 `chaos-ui/src/views/Sdk.vue`：
  1. **来源可视化**：每个 SDK 类型下展示其 `sources` 数组（repo/single 标签 + root 路径）。
  2. **切换语义修正**：仅当目标版本来自某个 `repo` 来源时才允许点击切换；`single` 来源仅展示、不可切换（标签置灰/禁用）。
  3. **来源/类型管理**：通过 `GET/POST /api/sdks/defs`、`PATCH/DELETE /api/sdks/defs/:name` 增删改 SDK 类型与 `sources`（支持动态增删 repo/single 来源项）。
- 新增前端 API 封装（在 `utils/api.ts` 或 Sdk.vue 内）调用上述 defs 接口。
- 前端读取 `GET /api/sdks` 的 `VersionList`/`CurrentVersion` 行为保持不变，仅补充来源信息展示。

## Impact

- 受影响文件：`chaos-ui/src/views/Sdk.vue`（主要）、可能新增 `chaos-ui/src/utils/sdk.ts` 封装。
- 后端契约（已落地）：`GET /api/sdks`、`GET /api/sdks/:type`、`PATCH /api/sdks/:type/switch`、`GET/POST /api/sdks/defs`、`PATCH/DELETE /api/sdks/defs/:name` 均已存在，前端仅消费。
- 不改动后端、不改动路由。纯前端增强。
- `App.vue` 侧边栏已有 `sdk` 菜单项，无需新增入口。
