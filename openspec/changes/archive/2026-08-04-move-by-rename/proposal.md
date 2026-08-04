# Change Proposal: 项目移动/回收站改用直接移动（rename）

## Why

当前 `MoveProjectFolder`（`fs.go:66`）实现为「复制源目录到目标 → 上层再删除源目录」，而非真正的移动。
这导致：
- 移动大型项目时产生整目录拷贝，耗时且占临时空间；
- 删除进回收站（`DeleteProject`）同样是「复制进回收站目录 → 再删原目录」，两次全量拷贝；
- 跨驱动器（cross-volume）时 `os.Rename` 失败，需回退到真正的复制+删除，但当前实现**无条件复制**，已偏离意图。

目标行为：移动项目和删除进回收站时，**直接移动顶层目录本身**（同一卷内 rename，跨卷才复制+清理），不预先复制。

## What Changes

- `MoveProjectFolder(oldAbs, newAbs)` 改为优先 `os.Rename`（同卷原子移动）；跨卷失败则回退 `copyDir + RemoveDirSafe`。
- `MoveProject` 和 `DeleteProject` 调用 `MoveProjectFolder` 后**不再额外调用 `MoveToRecycleBin(oldAbs)` / `RemoveDirSafe(oldAbs)`**（避免二次删除已不存在的源）。
- `MoveToRecycleBin` 保留，但仅用于跨卷回退路径或手动清理由本模块以外触发的回收站动作；移动/删除流程不再依赖它。

## Impact

- 受影响文件：`internal/project/fs.go`（`MoveProjectFolder`）、`internal/project/project.go`（`MoveProject`、`DeleteProject`、`RestoreProject` 配套回滚逻辑）
- API 契约不变：请求/响应字段、状态码保持一致；仅内部物理操作从「复制」变为「移动」。
- 失败语义变化：同卷移动为原子操作，不再有「复制成功但删源失败」的中间态。
