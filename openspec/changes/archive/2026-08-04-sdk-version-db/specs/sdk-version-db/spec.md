# Spec: SDK 版本管理（数据入库，按 SDK 类型组织 + JSON 来源数组）

## Purpose

将 SDK 来源的配置（SDK 类型、来源集合、当前启用版本、启用状态、备注）从硬编码代码迁移到数据库，使系统支持每个 SDK 类型下混合配置多个来源——**仓库**（父目录下子文件夹为各版本）与**单个版本**（一个目录即一个版本），并可在运行时增删多个来源与多个 SDK 类型；切换版本的底层文件系统机制保持不变。

## ADDED Requirements

### Requirement: SDK 来源存储于数据库，按 SDK 类型组织
系统 SHALL 将每个 SDK 类型持久化为 `SdkSource` 记录，**一行代表一个 SDK 类型**（如 jdk/maven/python），而非一个来源；记录包含 `Name`（唯一标识）、`Sources`（`jsonb` 来源数组）、`Current`、`Enabled`、`Note`，而不是写死在代码中。

#### Scenario: 启动时种子默认仓库
- **WHEN** 数据库为空且程序启动
- **THEN** 自动插入 jdk/maven/python/llama 四个 SDK 类型，各含一个 `Kind=repo`、`Root=D:\opt\xxx` 的来源，已存在则跳过不覆盖

#### Scenario: 读取仅来自数据库
- **WHEN** 请求 `GET /api/sdks`
- **THEN** 返回 map 的 key 为 SDK 类型 `Name`，内容由 DB 中 `Enabled=true` 且 `is_deleted=false` 的记录生成，不再引用任何代码内字面量 map

### Requirement: 每个类型支持混合多个来源（仓库与单个版本）
系统 SHALL 在 `Sources` JSON 数组中支持 `Kind=repo` 与 `Kind=single` 两种来源元素混合共存，每个元素含 `kind` 与 `root`（绝对路径）。

#### Scenario: 仓库来源版本列表来自子目录
- **WHEN** 某 `Kind=repo` 来源的 `root` 下存在若干子目录
- **THEN** 返回时实时扫描得到 `VersionList` 为这些子目录名（排除 `current` 与 `.current-version`）

#### Scenario: 单个版本来源无子目录扫描
- **WHEN** 某 `Kind=single` 来源的 `root` 为一个版本目录
- **THEN** `VersionList` 仅含 `filepath.Base(root)`，无磁盘扫描

#### Scenario: 同类型可同时含仓库与单版本
- **WHEN** 某 SDK 类型的 `Sources` 同时含 repo 与 single 元素
- **THEN** 两者版本列表合并返回，互不干扰

### Requirement: 当前启用版本以绝对路径唯一表示
系统 SHALL 以 `Current` 字段（绝对路径）记录该 SDK 类型当前启用的版本，`Current` 为单值，天然保证同一时刻只有一个版本处于启用状态。

#### Scenario: 仓库版本比对启用状态
- **WHEN** 返回某 repo 来源的版本列表
- **THEN** 绝对路径等于 `Current` 的子目录标为 `CurrentVersion`，其余未启用

#### Scenario: 单版本启用状态比对
- **WHEN** 返回某 single 来源
- **THEN** 其 `root` 等于 `Current` 时 `CurrentVersion` 为其目录名，否则为空

### Requirement: SDK 类型与来源可运行时管理
系统 SHALL 提供 CRUD 接口管理 SDK 类型及其 `Sources` 数组，支持新增、编辑、软删除与列出，且可添加多个 SDK 类型与多个来源。

#### Scenario: 新增 SDK 类型
- **WHEN** 提交 `POST /api/sdks/defs` 含合法 Name 与 Sources
- **THEN** 记录入库，Name 唯一索引防止重复

#### Scenario: 编辑来源数组
- **WHEN** 对 `PATCH /api/sdks/defs/:name` 提交新 Sources
- **THEN** 对应记录的 Sources 更新，其余字段不变

#### Scenario: 软删除 SDK 类型
- **WHEN** 调用 `DELETE /api/sdks/defs/:name`
- **THEN** `is_deleted` 置 true，后续列表与版本读取均不含该类型

#### Scenario: 列出来源
- **WHEN** 请求 `GET /api/sdks/defs`
- **THEN** 返回所有 `is_deleted=false` 的 SDK 类型（含 Enabled=false 的）

### Requirement: 仅仓库可切换版本
系统 SHALL 保持仓库切换版本时创建 `current` 符号链接并写入 `.current-version` 文件，并把类型的 `Current` 更新为目标版本绝对路径；单个版本不支持切换。

#### Scenario: 仓库切换版本来源 DB
- **WHEN** 调用 `PATCH /api/sdks/:name/switch` 且目标版本位于某 repo 来源的 `root` 下
- **THEN** 用 DB 中该 repo 来源的 `root` 定位目标子目录，重建 symlink 并写版本文件，同时把 `Current` 持久化为 `root/<version>` 绝对路径；Name 不存在返回 404

#### Scenario: 单个版本禁止切换
- **WHEN** 调用 `PATCH /api/sdks/:name/switch` 且请求版本仅能在 single 来源匹配
- **THEN** 返回 400，不执行任何文件系统操作
