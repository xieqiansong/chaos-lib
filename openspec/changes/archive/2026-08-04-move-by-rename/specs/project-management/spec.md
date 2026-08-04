# Spec Delta: Project Management (项目移动与回收站)

## ADDED Requirements

### Requirement: Direct move semantics
系统 SHALL 在移动项目与移入回收站时直接移动顶层目录（同卷 rename，跨卷复制+清理），不得预先全量复制。

#### Scenario: Same-volume move is atomic
- **WHEN** 项目源目录与目标在同一文件系统
- **THEN** 物理目录经 `os.Rename` 原子移动，无复制中间态

#### Scenario: Cross-volume move falls back to copy
- **WHEN** 项目移动跨文件系统（rename 失败 EXDEV）
- **THEN** 回退为 `copyDir + 删除源`，行为与历史一致

#### Scenario: No double deletion
- **WHEN** 移动或移入回收站完成
- **THEN** 源目录已被移动走，调用方不再额外删除源
