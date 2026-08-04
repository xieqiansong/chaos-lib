# Tasks

## 1. 改写 MoveProjectFolder 为直接移动
- [ ] 在 `fs.go` 中将 `MoveProjectFolder` 改为优先 `os.Rename`，跨卷失败回退 `copyDir + RemoveDirSafe(oldAbs)`
- [ ] 保留目标已存在校验、父目录创建、源不存在校验
- [ ] 确认复制回退失败时清理 `newAbs`

## 2. 清理 MoveProject 的二次删除
- [ ] 删除 `MoveProject` 中 `moved` 分支下的 `MoveToRecycleBin(oldAbs)` 调用
- [ ] 确认 `tx.Rollback()` 后回滚 `newAbs` 的 `RemoveDirSafe` 仍保留

## 3. 清理 DeleteProject 的二次删除
- [ ] 删除 `DeleteProject` 末尾的 `RemoveDirSafe(oldAbs)`（源已随 `MoveProjectFolder` 移动）
- [ ] 确认非回收站组（永久删除分支）逻辑不受影响

## 4. 编译与冒烟
- [ ] `go build ./...` 无编译错误
- [ ] 确认 `MoveToRecycleBin` 仍被引用（如未被引用可标注但不删除，保留为独立能力）
