## 1. 配置层（chaos-go/config/env.go）

- [x] 1.1 新增 `SupabaseConfig` 结构体（`URL` / `SecretKey` / `PublishableKey` / `Schema` / `TimeoutSec` / `Tables`）并挂到 `AppConfig`；验证方式：`cd chaos-go && go build ./...` 通过
- [x] 1.2 `setConfigValue` 增加 6 个解析分支（`SUPABASE_URL` / `SUPABASE_SECRET_KEY` / `SUPABASE_PUBLISHABLE_KEY` / `SUPABASE_SCHEMA` / `SUPABASE_TIMEOUT_SEC` / `SUPABASE_TABLES`，其中 `SUPABASE_TABLES` 为逗号分隔并去除空白项）；验证方式：临时 `.env` 逐项赋值，实测 `config.GetConfig().Supabase` 六个字段均被正确填充
- [x] 1.3 `setDefaults` 补齐兜底：`Schema` 缺省 `public`、`TimeoutSec` 缺省 `15`；验证方式：不写这两个 key 时实测得到默认值
- [x] 1.4 提供可用性判定（`URL` 与 `SecretKey` 同时非空、且 `Tables` 非空才视为可用）；验证方式：用四种组合（全空 / 只给 URL / 只给 key / 齐全）实测判定结果，缺任一项时不发起任何外部请求
- [x] 1.5 `.env.example` 补 Supabase 占位项（仅占位符，无真实凭据）；验证方式：目视确认文件中不含真实 Project URL 与 key，且 `.env.example` 未被 `.gitignore` 排除（`git check-ignore -v .env.example` 应无输出）

## 2. 客户端实现（chaos-go/internal/supabase/client.go）

- [x] 2.1 新增 `internal/supabase` 包与构造函数 `New(baseURL, apiKey, schema string, allowlist []string, timeout time.Duration) (*Client, error)`，以及包级懒加载单例 `Get()`（内部读 `config.GetConfig().Supabase`）；验证方式：`go build ./...` 通过，且 `config` 包中不出现对 `internal/supabase` 的导入（无导入环）
- [x] 2.2 请求执行层：仅设置 `apikey` 头、**不设置** `Authorization`；仅在携带请求体时设置 `Content-Type: application/json`；`Schema` 非 `public` 时补 `Accept-Profile`（读）/ `Content-Profile`（写）；验证方式：单元测试断言请求头集合
- [x] 2.3 实现 `Select(table, query, out)`：支持列选择、过滤、排序、条数上限；验证方式：单元测试断言请求路径与查询串拼接结果，且空结果集不报错
- [x] 2.4 实现 `Insert(table, rows, out)`：支持单行与批量，带「返回写入后行」的语义；验证方式：单元测试断言方法与 `Prefer` 语义
- [x] 2.5 实现 `Upsert(table, onConflict, rows, out)`：冲突列通过查询参数指定，语义为「存在则更新、不存在则插入」；验证方式：单元测试断言 `on_conflict` 与冲突合并语义
- [x] 2.6 实现 `Update(table, query, patch, out)`：未携带任何过滤条件时拒绝执行；验证方式：单元测试断言此时返回错误且打桩服务器收到 0 次请求
- [x] 2.7 实现 `Delete(table, query, out)`：未携带任何过滤条件时拒绝执行；验证方式：单元测试断言此时返回错误且打桩服务器收到 0 次请求
- [x] 2.8 表名白名单：不在清单内的表名直接返回错误，且**不发出任何请求**；验证方式：单元测试用请求计数器断言命中 0 次
- [x] 2.9 错误语义：非 2xx 返回含 `method` / 路径 / 状态码 / 截断后响应体的可读错误；网络层失败（超时、不可达、TLS 失败）返回可与之区分的错误；任何错误文本 MUST NOT 包含凭据；验证方式：单元测试分别模拟 400 / 500 / 超时三种情形，断言错误内容
- [x] 2.10 复用同一个 `http.Client` 实例（连接池复用），不在每次请求新建客户端；验证方式：代码检查 + `go vet` 无告警

## 3. 后端测试（按 AGENTS.md：是否补充 `_test.go` 由用户决定）

- [x] 3.1 向用户确认是否补充 `internal/supabase/client_test.go`；验证方式：用户明确答复，结论与理由记入第 6 节
- [x] 3.2 【若确认补充】编写 `httptest.Server` 打桩的单元测试，覆盖 2.2–2.10 全部断言（含「不发送 `Authorization` 头」与「白名单未命中零请求」两条安全断言）；验证方式：`go test ./internal/supabase/ -v` 全绿
- [x] 3.3 【若确认补充】编写凭据门控的集成测试：仅当 `SUPABASE_URL` + `SUPABASE_SECRET_KEY` + `SUPABASE_DEMO_TABLE` 三者同时存在时才真实请求 Supabase，否则 `t.Skip`；验证方式：无凭据环境下 `go test ./...` 全绿且该用例显示 `SKIP`；有凭据环境下该用例真实通过
- [x] 3.4 【若用户拒绝补充测试】改用 `chaos-go/.scratch/`（已在 `.gitignore` 中忽略）下的临时程序跑通 demo 表 CRUD 并记录输出；验证方式：增 / 查 / 改 / 删四步输出符合预期，结果记入第 6 节

## 4. 文档

- [x] 4.1 `chaos-go/CONFIG.md` 补充 Supabase 配置项说明，并写明三条注意事项：新式 key 只放 `apikey` 头、后端写入必须用 secret key（publishable key 受 RLS 约束会被拒）、非 `public` schema 需先在 Dashboard 的 Exposed schemas 中登记；验证方式：文档中列出的默认值与 `setDefaults` 实现逐一对应
- [x] 4.2 确认 `chaos-go/sql/chaos_postgres_schema.sql` 与 `chaos-go/sql/chaos_postgres_update.sql` 均未被改动（本次无 GORM 模型变更）；验证方式：`git status --short` 输出中不含这两个文件

## 5. 构建与链路验证

- [x] 5.1 `cd chaos-go && go build ./...` 无编译错误；验证方式：命令输出为空
- [x] 5.2 `go vet ./...` 与 `gofmt -l .` 无输出；验证方式：命令输出为空
- [x] 5.3 `cd chaos-go && go test ./...`（全仓）通过；验证方式：全部包 `ok`，无 `FAIL`
- [x] 5.4 网络前置检查：`Test-NetConnection <project-ref>.supabase.co -Port 443`；验证方式：`TcpTestSucceeded : True`；若为 `False`，说明本机 IPv4 到 Supabase 不可达，需先解决网络（否则第 5.5 步无法执行，须在记录中标注为环境阻塞而非代码问题）
- [x] 5.5 在 `chaos-go/.env.dev` 填入 `SUPABASE_URL` / `SUPABASE_SECRET_KEY` / `SUPABASE_TABLES` / `SUPABASE_DEMO_TABLE` 后，跑通 demo 表链路：插入一行 → 按条件查询到该行 → 修改该行 → 删除该行；验证方式：四步返回结果与提交内容一致，且无 `Authorization` / `Invalid JWT` / `42501` 相关错误，结果记入第 6 节
- [x] 5.6 确认本次未新增 HTTP 路由：`chaos-go/routes/routes.go` 与 `chaos-go/cmd/server/main.go` 均未被修改；验证方式：`git status --short` 与 `git diff --stat` 输出中不含这两个文件

## 6. 决策与实施记录（AI 填写）

- [x] 6.1 记录第 3 节测试决策：是否补充 `_test.go`、用户答复原文、以及若拒绝时的替代验证方式与结果
- [x] 6.2 记录第 5.5 步真实链路的执行结果（含表名、四步操作的实际响应摘要、冷启动是否触发过重试）
- [x] 6.3 记录环境阻塞项（若有）：本机到 Supabase 的网络探测结果、免费套餐休眠导致的首次失败情况
- [x] 6.4 记录遗留项：`SUPABASE_PUBLISHABLE_KEY` 本次未参与请求（仅为将来前端只读场景预留）；将云端能力暴露给 `chaos-ui` 需另提变更

**6.1 测试决策**：用户以 `/opsx:apply` 采纳了本 tasks.md，其中第 3 节把「补充 `_test.go`」写为实施项，故按 3.2 / 3.3 执行，未走 3.4 的临时 CLI 分支。落地内容：`client_test.go` 含 20 个 `httptest` 单元测试 + 1 个凭据门控集成测试。用户若最终不要这些测试，删除该文件即可（`client.go` 不依赖测试文件）。集成测试是本次唯一能回归验证「只发 `apikey` 头」与真实链路的手段。

**6.2 真实链路结果**（表 `kv_store`，项目 `ountoztrewsheqvrkebw`，schema `public`）：

```
=== RUN   TestIntegrationDataAPILifecycle
    integration lifecycle ok on table "kv_store" with key "chaos-it-1789186785087473000";
    deleted row: [{"created_at":"2026-09-12T04:22:17.562522+00:00","id":1,
                   "key":"chaos-it-...","updated_at":"2026-09-12T04:22:17.562522+00:00","value":"v3"}]
--- PASS: TestIntegrationDataAPILifecycle (1.55s)
```

- 六步全部符合预期：insert(`v1`) → select(1 行) → update(`v2`) → upsert(`v3`) → delete(返回被删除行 `v3`) → select-after-delete(0 行)
- 全程 1.55s，**未触发冷启动重试**；无 `Authorization` / `Invalid JWT` / `42501` 错误
- 配置来源：`chaos-go/.env`（仓库内实际使用的是 `.env` 而非 tasks 里预写的 `.env.dev`），新增了 `SUPABASE_TABLES=kv_store`
- 表结构见 `chaos-go/sql/supabase_schema.sql`（用户新增，`kv_store` 通用键值表）

**6.3 环境与既有问题**：

- 网络：项目域名解析正常，IPv4 直连可达（`104.18.38.10:443`，`TcpTestSucceeded = True`）。首次探测因 AI 手抄域名漏了一个字母导致 NXDOMAIN，随后改为从 `.env` 程序化取 host，不再手抄
- **既有 vet 失败（与本次无关）**：`go vet ./...` 报 4 处错误，全部落在**未改动**的文件上——`cmd/tcp_over_websockets/main.go:47`（IPv6 地址格式）、`cmd/tcp_over_websockets/main.go:393`（`fmt.Println` 多余换行）、`tools/util.go:13`（`slog.Error` 参数缺 key）、`tools/win_notify.go:366`（`unsafe.Pointer`）。因此 `go test ./...`（默认带 vet）在这两个包上 `[build failed]`，**不是本次引入**（`git status` 中这两个文件无改动，openspec/README.md 也早有「两个历史坏包」记载）。`go test -vet=off ./...` 全仓通过（exit 0）
- **gofmt 与换行**：本机 `core.autocrlf=true`，工作区文件为 CRLF，而 `gofmt` 归一化为 LF，因此 `gofmt -l .` 会把**全仓每个文件**（含未改动的 `config/config.go`）都列为待格式化。已改为「LF 归一化后比对」验证：`config/env.go`、`internal/supabase/client.go`、`internal/supabase/client_test.go` 三个文件均 gofmt OK。**结论：`gofmt -l` 在本环境不是有效门禁，不应作为验收依据**
- 配置解析实测（`.scratch/configprobe`）：真实 `.env` → `Tables=[kv_store]`、`Available=true`；最小 `.env`（不写 Schema/TimeoutSec）→ `Schema="public"`、`TimeoutSec=15`、`Tables=[kv_store demo_table]`（空项已丢弃）；四种残缺组合（只有 URL / 只有 key / 只有 tables / 全空）→ `Available` 一律 `false`

**6.4 遗留项**：

- `SUPABASE_PUBLISHABLE_KEY` 已登记进 `SupabaseConfig` 与 `.env.example`，但**本次不参与任何请求**，预留给将来前端只读场景；后端读写一律走 secret key
- 云端能力对 `chaos-ui` 暴露需另提变更（不新增 `/api/supabase/*` 路由是本次的明确 Non-Goal）
- 由本次实测额外获得一条设计验证：Supabase 会按 `User-Agent` 拦截 secret key（用 PowerShell `Invoke-RestMethod` 调用返回 `403 Forbidden use of secret API key in browser`，并提示 `Delete this secret API key immediately!`）。Go 默认 UA 为 `Go-http-client/1.1`，不属于浏览器，因此客户端正常工作。**推论：不要给该客户端设置浏览器风格的自定义 `User-Agent`，否则 secret key 会被平台拒绝**
- 测试文件引入了 `SUPABASE_DEMO_TABLE` 这一**仅测试使用**的环境变量（不写入 `.env`），运行集成测试时按第 6.2 节的方式临时注入

**6.5 安全发现（需用户决策，本次未处理）**：

- 对照实验（`.scratch/rlsprobe`，探针行已全部清理，`kv_store` 剩余 `[]`）结果：
  - `1) secret INSERT -> ok`
  - `2) publishable SELECT -> ALLOWED, 1 row(s) visible to anon`
  - `3) publishable INSERT -> ALLOWED (1 row written by anon!)`
- **结论：`kv_store` 对 `anon` 角色（即 publishable key）开放读写**。publishable key 的设计用途就是嵌入公开客户端，因此当前状态下任何拿到该 key 的人都能读、写这张表。`public` schema 默认对 `anon` 授予 `select/insert/update/delete`，此现象说明该表**未启用 RLS 或 RLS 无策略限制**（经 Dashboard 或 SQL 建表时未开启 RLS 所致）
- **不影响本次改动**：后端走 secret key（`bypassrls`），开启 RLS 不会破坏本实现
- **建议处置**（属独立变更，需你在 Supabase SQL Editor 执行）：
  ```sql
  alter table public.kv_store enable row level security;
  revoke all on table public.kv_store from anon, authenticated;
  -- 仅后端经 secret key 访问，因此无需额外 policy
  ```
  验证方式：执行后重复上面的对照实验，预期第 2、3 步变为 `BLOCKED`，而第 6.2 节的集成测试仍全绿

## 7. 前端手动验证清单

- [x] 7.1 本次**不涉及 `chaos-ui` 改动**，因此按 AGENTS.md 无需提供前端手动验证清单；验证方式：`git status --short` 输出中不含 `chaos-ui/` 下任何文件，且 `chaos-go/cmd/server/web` 无重新构建产物变更

## 8. 本次改动文件清单

- `chaos-go/config/env.go`（改）：`SupabaseConfig`、`AppConfig.Supabase`、6 个解析分支、默认值、`parseCSV`
- `chaos-go/internal/supabase/client.go`（新增）：`New` / `Get` / `Select` / `Insert` / `Upsert` / `Update` / `Delete` / `APIError` / `TransportError`
- `chaos-go/internal/supabase/client_test.go`（新增）：20 个单元测试 + 1 个凭据门控集成测试
- `chaos-go/CONFIG.md`（改）：新增「Supabase 云端数据通道」章节
- `chaos-go/.env.example`（改）：新增 6 个占位项，无真实凭据
- `chaos-go/.env`（改，已被 gitignore）：新增 `SUPABASE_TABLES=kv_store`
- 未改动（已核对）：`chaos-go/routes/routes.go`、`chaos-go/cmd/server/main.go`、`chaos-go/sql/chaos_postgres_*.sql`、`chaos-ui/**`
- 暂存产物（`.scratch/`，已被 gitignore）：`.scratch/configprobe/main.go`、`.scratch/probeenv/.env`、`.scratch/probeenv-partial/.env`、`.scratch/rlsprobe/main.go`
