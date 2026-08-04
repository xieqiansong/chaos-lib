# Design: 直接移动实现

## 决策

1. **同卷优先 rename**：`os.Rename(oldAbs, newAbs)` 在 Windows / Linux 同文件系统上为 O(1) 原子操作，
   不复制数据。成功即完成；无需后续 `RemoveDirSafe(oldAbs)`。
2. **跨卷回退复制**：`os.Rename` 返回 `EXDEV`（跨设备）时，回退到 `copyDir + RemoveDirSafe(oldAbs)`，
   行为与当前复制一致，但只在必要时发生。
3. **移动后清理源的责任单一化**：`MoveProjectFolder` 成功后，源目录已不存在。调用方（`MoveProject` /
   `DeleteProject`）**移除**原先的 `MoveToRecycleBin(oldAbs)` / `RemoveDirSafe(oldAbs)` 调用，避免误删或报错。
4. **失败回滚**：跨卷回退路径下若复制失败，已产生的 `newAbs` 由 `MoveProjectFolder` 内部 `RemoveDirSafe(newAbs)` 清理；
   DB 事务失败时，调用方回滚 DB 即可（物理目录留在原处，符合「移动失败即不动」语义）。
5. **`MoveToRecycleBin` 保留**：作为独立能力（如 UI 手动「送入系统回收站」），不纳入移动/删除主流程。

## 实现要点

```go
func MoveProjectFolder(oldAbs, newAbs string) error {
    if oldAbs == newAbs { return nil }
    // 校验源目录、目标不存在、建父目录（同现有逻辑）
    if err := os.Rename(oldAbs, newAbs); err == nil {
        return nil // 同卷原子移动
    }
    // 跨卷回退：复制 + 删源
    if cerr := copyDir(oldAbs, newAbs); cerr != nil {
        _ = RemoveDirSafe(newAbs)
        return fmt.Errorf("移动目录失败: %v", cerr)
    }
    _ = RemoveDirSafe(oldAbs)
    return nil
}
```

`MoveProject` / `DeleteProject`：
- 删除 `moved` 判断后的 `MoveToRecycleBin(oldAbs)` 调用；
- 删除 `DeleteProject` 末尾的 `RemoveDirSafe(oldAbs)`（源已随 `MoveProjectFolder` 移动走）；
- `tx.Rollback()` 后的 `RemoveDirSafe(newAbs)` 回滚保留（目录已不存在时 `RemoveDirSafe` 安全 no-op）。

## 风险

- 跨卷移动仍是复制，大目录耗时——但仅在跨驱动器时发生，与原行为一致且更快（少一次删源）。
- Windows 下若目录被占用（编辑器/git），`os.Rename` 可能失败，回退复制路径可覆盖。
