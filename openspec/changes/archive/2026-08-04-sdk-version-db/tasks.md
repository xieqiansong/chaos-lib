# Tasks

## 1. 新增 SdkSource 模型与种子
- [x] 在 `internal/proxy/sdk.go` 定义 `SdkSource` 结构体（`Name` 唯一索引、`Sources []byte` jsonb、`Current`、`Enabled`、`Note`、`IsDeleted`）与 `SdkSourceItem`（`Kind`/`Root`）
- [x] 在包 `init()` 中 seed 默认 4 个 SDK 类型（jdk/maven/python/llama），各含一个 `kind=repo`、`root=D:\opt\xxx` 的来源，已存在则跳过
- [x] 在 `cmd/server/main.go` 的 `AutoMigrate()` 登记 `&proxy.SdkSource{}`

## 2. 改写读取/切换逻辑走 DB（区分 repo / single）
- [x] 新增 `getSdkInfo(s SdkSource) SdkInfo`：遍历 `Sources`；single 仅返回 `[Base(root)]` 并比对 `Current`；repo 扫描 `root` 子目录（排除 `current`/`.current-version`）并比对 `Current` 绝对路径
- [x] `GetSdkVersions` 从 DB 读 SDK 类型，仅对 `Enabled=true` 返回 `getSdkInfo`，map key 用 `Name`
- [x] `UpdateSdkVersion` 用 `Name` 取类型；遍历 `Sources` 仅对 `repo` 项定位 `root/version`，重建 symlink + 写 `.current-version`，并把该类型 `Current` 更新为 `filepath.Join(repo.Root, version)` 持久化；非 repo / 版本不存在返回 400/404

## 3. 删除死代码
- [x] 删除 `services/sdk.go`（确认 `services.GetSdk*` 零引用）

## 4. 新增 SDK 类型 CRUD 接口
- [x] `ListSdkSources` GET `/api/sdks/defs`
- [x] `CreateSdkSource` POST `/api/sdks/defs`（校验 `Sources` 元素 `Kind ∈ {repo,single}`，`Name` 唯一）
- [x] `UpdateSdkSource` PATCH `/api/sdks/defs/:name`
- [x] `DeleteSdkSource` DELETE `/api/sdks/defs/:name`（软删除）
- [x] 在 `routes/routes.go` 注册上述路由（统一 `/api` 前缀）

## 5. 编译与校验
- [x] `go build ./...` 无编译错误
- [x] `openspec validate --changes` 通过
