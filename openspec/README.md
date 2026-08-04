# OpenSpec 使用手册（给人的速查）

> AI 按 `config.yaml` + 本目录结构自动走流程，无需人指导。
> 本手册是给你（人）看的：什么时候该做什么、AI 会怎么配合、容易忘的坑。

## 一句话心智模型

**Spec 是契约，不 review 代码 diff；测试是底气，必须真跑；变更用完要归档。**

---

## 全流程 5 步

| 步骤 | 你做啥 | AI 做啥 | 完成标志 |
|------|--------|---------|----------|
| 0. 初始化 | 装 CLI：`npm i -g @fission-ai/openspec`，`openspec init --tools codebuddy` | — | `openspec/` 出现 config.yaml、specs/、changes/ |
| 1. 写/更新 Spec | （可选）先定功能规格 | 写 `specs/<module>/spec.md`，含 Purpose/Requirements/Scenario | `openspec validate --specs` 通过 |
| 2. 提需求 | 说一句 `/opsx:propose "你的需求"` | 生成 changes/<name>/ 四件套（proposal/design/tasks/specs） | `openspec validate --changes` 通过 |
| 3. 落地 | 说 `/opsx:apply` | 按 tasks.md 改代码 | `go build ./...` 通过 |
| 4. 补测试+跑 | 提醒"实际跑测试" | 写测试，**真跑 `go test ./...`**（全仓才算数） | 测试全绿 |
| 5. 归档 | **确认可用后** | AI 主动提醒，你/AI 跑 `/opsx:archive` | change 从 open 列表消失 |

---

## 三个铁律（最容易被忘）

1. **Spec 是契约** — 不要靠肉眼 review AI 生成的 diff，靠 spec 的 Requirements + Scenario 来验收。
2. **测试必须实际跑** — 只写不跑 = 没写。且要 `go test ./...`（全仓），不是只跑某个包。
3. **全仓 `go test ./...` 才算数** — 单包绿可能掩盖别处 broken build（本次就抓到两个历史坏包）。

---

## 本次实战时间线（参考样例）

- 写 `specs/task-plan/spec.md` → 校验卡了 3 次（缺 Purpose / 缺 SHALL / 缺 Scenario），补正后过
- `/opsx:propose "移动直接 rename 不复制"` → 生成 `changes/move-by-rename/`
- `/opsx:apply` → 改 `fs.go`（MoveProjectFolder 优先 os.Rename，跨卷回退 copy+rm）、`project.go`（去冗余删源）
- 补 `fs_test.go` 8 个测试，逐字节校验子文件内容 → 抓到测试自身 bug + 全仓 2 个历史坏包 → 修复后全绿
- **待办**：`/opsx:archive move-by-rename`

---

## 常见坑

- **改完就走，忘 archive** → change 一直 open，下次 propose 污染列表。让 AI 在确认时提醒你。
- **spec 校验不过** → 缺 `## Purpose` / Requirements 里没 `SHALL`/`MUST` / 没 `#### Scenario:` 块，逐个补。
- **测试只查存在性** → 移动/复制类一定要逐字节比对子文件（用 snapshotTree + assertTreeMatch 思路）。
- **`openspec init` 无输出** → 加 `--no-animation` 重试。

---

## 命令速记

```bash
openspec init --tools codebuddy --no-animation   # 首次初始化
openspec validate --specs                        # 校验规格
openspec validate --changes                      # 校验变更提案
/opsx:propose "需求描述"                         # 提变更
/opsx:apply                                      # 落地代码
/opsx:archive <change-name>                      # 归档（用完必做）
```
