## Context

见 `proposal.md - Why`。与本次相关的现状与约束：

- `chaos-go` 现有数据通道只有 GORM：`config.GetDB()` 单例（`config/config.go`）按 `cfg.Database.Type` 在 `glebarez/sqlite` 与 `gorm.io/driver/postgres` 之间二选一；`config.AutoMigrate` 在 `cmd/server/main.go` 登记全部模型；`chaos-go/sql/chaos_postgres_schema.sql` 是只读基准快照，模型变动需追加 `chaos_postgres_update.sql`。
- 配置加载在 `config/env.go`：`flag -env` / `APP_ENV` 决定读 `.env` 或 `.env.{dev,prod}`，解析器按首个 `=` 切分 `KEY=VALUE`，值两端去引号，不做额外转义；未识别 key 静默忽略；`setDefaults` 兜底。新增配置项需同时改 `setConfigValue` 与 `setDefaults`。
- 本机网络为 IPv4-only。Supabase 直连（`db.<ref>.supabase.co:5432`）免费套餐仅 IPv6；共享池（`aws-<n>-<region>.pooler.supabase.com`）提供 IPv4，但会引入 Supabase 特有的连接约束（`<n>` 索引无法从 region 推导、用户名须为 `postgres.<ref>`、事务模式须关预编译语句、连接数须下调到池大小之内）。
- 凭据形态：Supabase 正在用新式 key 替代旧的 `anon` / `service_role` JWT，官方计划 2026 年底弃用旧 key。新式 `sb_publishable_*`（等价 `anon`，受 RLS 约束）与 `sb_secret_*`（等价 `service_role`，绕过 RLS）**都不是 JWT**，官方要求只放 `apikey` 头。
- 硬规则：不搞依赖注入，基础设施用包级单例；路由统一在 `routes.SetupRouter` 注册。本次按用户要求**不接路由**。

## Goals / Non-Goals

**Goals:**

- 用一条 `internal/supabase` 包把 PostgREST 的增删改查收敛成 5 个方法，且与既有 GORM 体系完全隔离（可整包删除而不影响任何现有功能）。
- 凭据传输方式一次做对：只发 `apikey` 头，杜绝新式 key 走 `Authorization: Bearer` 导致的 `Invalid JWT`。
- 在不接 HTTP 路由的前提下，仍能验证真实链路（用 Go 测试作为验证载体）。
- 客户端可被单元测试完全覆盖（可注入 baseURL，不依赖真实网络）。

**Non-Goals:**

- 不接 Supabase Auth / Storage / Realtime / Edge Functions / GraphQL。
- 不引入 Postgres 直连或连接池，不改 `DatabaseConfig` 与 `GetDSN()`。
- 不新增或修改任何 GORM 模型，不触碰 `AutoMigrate` 登记与 `chaos_postgres_update.sql`。
- 不做本地与云端的双向同步 / 冲突解决。
- 不新增 HTTP 路由，不动 `routes.go` 与 `chaos-ui`。
- 不做自动重试与熔断。
- 不支持分页总数（`count=exact`）与关联嵌套查询。

## Decisions

### D1: 用 Data API（PostgREST）而不是 Postgres 直连或共享池

- **为什么**：直连在免费套餐下只有 IPv6，本机 IPv4-only，路线直接不通。共享池虽可走 IPv4，但代价是把一整套 Supabase 连接约束搬进代码与 `.env`：pooler 主机名里的 `aws-<n>` 索引无法从 region 推导（必须人工从 Dashboard 复制）、用户名要写成 `postgres.<project-ref>`、事务模式必须关预编译语句、连接数要下调到池大小之内。随之而来的是 `GetDSN()` 需要新增 `URL` / `search_path` / `query_exec_mode` 等分支，`GetDSN()` 的契约被 Supabase 反向污染。
- **Data API 的优势**：纯 HTTPS，IPv4 天然可用；凭据是可按服务签发、可单独轮换的 secret key，而不是 Postgres 超级用户口令；权限面比「整库连接」小得多。
- **备选**：共享池 · 会话模式（结论：可用但被否，理由同上，且留作将来若确需 SQL 能力时的备选，届时应以独立变更提出）。

### D2: 凭据只放 `apikey` 头，且客户端手写

- **为什么**：`sb_publishable_*` / `sb_secret_*` 不是 JWT。官方明确要求「Send publishable and secret keys on the `apikey` header only」，并指出放入 `Authorization: Bearer` 会被下游按 JWT 解析而失败。
- **备选一**：社区 SDK `github.com/supabase-community/supabase-go`。已核对其 `client.go`：`NewClient` 默认构造 `Authorization: Bearer <key>` **和** `apikey: <key>` 两个头，正撞在上面这条规则上；虽然可用 `options.Headers` 覆盖 `Authorization`，但覆盖成一个非 JWT 字符串后行为不受控，且引入了一棵依赖树。
- **备选二**：继续用旧的 `anon` / `service_role` JWT 以兼容现有 SDK。结论：不引入即将被弃用的凭据形态。
- **结论**：用标准库 `net/http` 手写。需求只有 5 个方法，PostgREST 是纯 REST，没有值得为之引入依赖的复杂度；这也与项目「全手写、不引框架」的既有风格一致。
- **代价**：将来若要 Auth / Storage / Realtime，需要各自实现或届时单独评估依赖。

### D3: 后端读写用 secret key，不用 publishable key

- **为什么**：RLS 一旦启用且无策略，publishable key 读到空集、写入被拒（`42501`）。secret key 具备 `bypassrls`，只要请求不携带用户 access token 就绕过 RLS，因此无需为每张表编写 `select` / `insert` / `update` / `delete` 四条策略。
- **备选**：publishable key + 逐表 RLS 策略。被否：N 张表 × 4 条策略的维护成本，且 publishable 是公开凭据——谁能拿到就等于谁能写数据，与「个人工具箱的数据只归自己」冲突。
- **附带收益**：secret key 在浏览器环境会被平台按 `User-Agent` 拦为 401，天然无法泄漏到前端；publishable key 则可以在将来安全地用于前端只读场景。

### D4: 表名白名单放在客户端层

- **为什么**：PostgREST 的表名是 URL 路径段，不是 SQL 拼接，注入风险低；但它仍是外部可控字符串，且一旦将来接上路由，「任意表可读写」会直接变成攻击面。在客户端入口做显式允许清单，未命中直接拒绝且不发请求，把收敛点固定在最早的边界上。
- **实现**：允许清单由配置项提供（逗号分隔），为空时该通道视为不可用；表名 MUST 精确匹配清单项。
- **与本变更的关系**：本次不接路由，白名单暂无外部输入来源，但它同时充当「demo 表名」的唯一来源，避免把表名散落在代码里。

### D5: 单例放在 `internal/supabase`，依赖方向单向

- **方案**：`internal/supabase` 提供包级懒加载单例 `Get()`，内部读 `config.GetConfig().Supabase`。依赖方向是 `internal/supabase → config`，无导入环。
- **备选**：把单例做成 `config.GetSupabase()` 以严格对齐「`config.GetDB()` 单例」这条硬规则。被否：那会让依赖方向变成 `config → internal/supabase`，而 `internal/supabase` 又需要 `config` 的配置类型，形成导入环；规避办法是让 `New` 只接受基本类型参数，反而更绕、可读性更差。
- **一致性**：调用侧的形态（`supabase.Get()`）与 `config.GetDB()` 完全对称，硬规则的**意图**（不搞注入、用包级单例）被保留。
- **可测性**：单例之外同时保留导出的 `New(baseURL, apiKey, schema string, allowlist []string, timeout time.Duration)` 构造函数，测试直接构造实例并指向 `httptest.Server`，不走单例、不依赖真实网络。

### D6: 超时与重试策略

- **超时**：`http.Client{Timeout: SUPABASE_TIMEOUT_SEC}`，默认 15s。免费套餐项目休眠后首个请求可能很慢，超时过短会把「冷启动」误报为故障。
- **重试**：首版不自动重试。理由：写操作重试有重复写入风险（除非调用方使用 upsert + 幂等键），而「休眠导致首次失败」这种场景由调用方显式重试语义更清晰。将来若确认需要，再以独立变更引入「仅对读操作重试」。
- **连接复用**：使用同一个 `http.Client` 实例复用连接池，不在每次请求新建客户端。

### D7: 错误语义

- 非 2xx 状态码 → 返回带有 `method`、`path`、状态码与服务端原始响应体的错误；响应体做长度截断避免日志爆炸。
- 网络层失败（`*url.Error` / 超时）→ 返回与「服务端返回错误」可区分的错误，便于调用方判断是否值得重试。
- 错误文本 MUST NOT 包含请求头与凭据；错误里只出现路径与状态码。

### D8: 不接路由的前提下如何验证链路

- **单元测试**：`httptest.Server` 打桩，断言（a）请求头只含 `apikey`、不含 `Authorization`；（b）路径与查询串拼接正确（`select` / 过滤 / `order` / `limit` / `on_conflict`）；（c）`Prefer` 语义正确；（d）未在白名单中的表名在发出请求前即被拒绝；（e）非 2xx 会转成错误。这些随 `go test ./...` 默认运行，不需要任何凭据。
- **集成测试**：仅当 `SUPABASE_URL` + `SUPABASE_SECRET_KEY` + `SUPABASE_DEMO_TABLE` 三者同时存在时才真实请求 Supabase，否则 `t.Skip`。这样「demo 表 CRUD 打通链路」可以在不暴露 HTTP 接口的前提下完成验证，也不会让 `go test ./...` 在没有凭据的机器上失败。
- **备选**：加一个临时 CLI（`cmd/` 或 `.scratch/`）。被否：会比测试多一份需要维护和清理的代码，且验证结果不可回归。

### D9: 配置项纳入既有加载机制，不加独立开关

配置项：`SUPABASE_URL`、`SUPABASE_SECRET_KEY`、`SUPABASE_PUBLISHABLE_KEY`、`SUPABASE_SCHEMA`、`SUPABASE_TIMEOUT_SEC`、`SUPABASE_TABLES`（白名单，逗号分隔）。

- 「可用」的判定是 `SUPABASE_URL` 与 `SUPABASE_SECRET_KEY` 同时非空 —— 不再新增 `SUPABASE_ENABLED` 之类的开关，减少概念。
- `SUPABASE_SCHEMA` 缺省 `public`。若使用非 `public` schema，请求需要额外携带 `Accept-Profile`（读）与 `Content-Profile`（写）两个头，且该 schema 必须已在 Dashboard 的 Exposed schemas 中登记；这一点写入 `CONFIG.md` 作为注意事项。
- `SUPABASE_PUBLISHABLE_KEY` 本次不参与请求，仅登记在配置中，供将来前端只读场景使用（避免将来又要动配置结构）。
- 既有解析器按首个 `=` 切分且不做转义，因此 URL 中的 `://`、`?`、`&` 与 key 中的 `_`、大写字母都不会被破坏 —— 无需修改解析逻辑。

## Risks / Trade-offs

- [本机 IPv4 到 `*.supabase.co:443` 可能不可达，代码写完也验证不了] → 落地前先做 `Test-NetConnection <project-ref>.supabase.co -Port 443`；不可达则先解决网络/代理，本变更的集成测试会以 `t.Skip` 形式静默通过，属已知盲区，需在验证记录里显式说明。
- [误用 publishable key 做写入，表现为 `42501` 或读取恒为空] → D3 明确后端只用 secret key；spec 里写入失败 MUST NOT 静默成功；`CONFIG.md` 写清两者用途差异。
- [免费套餐项目休眠，首个请求超时，被误判为「链路不通」] → 超时默认放大到 15s；错误语义区分网络类与服务端类；验证记录里注明「冷启动需重试一次」。
- [仓库公开，secret key 或真实 Project URL 被提交] → 只写入 `.env.dev` / `.env.prod`（`.gitignore` 已有 `**/.env.*` 且 `!**/.env.example` 反向放行模板）；`.env.example` 只放占位符；错误与日志不回显凭据。
- [表名白名单是一份需要人工维护的清单，将来接路由时容易漏配] → 白名单是唯一的表名来源（含 demo 表），未命中直接拒绝；在 `tasks.md` 中写明「将来接路由时必须复用同一白名单」，作为后续变更的检查项。
- [手写客户端意味着将来要用 Auth / Storage / Realtime 时得重新造轮子] → 明确列为 Non-Goal；届时应作为独立变更重新评估「手写 vs 引 SDK」，而不是现在提前引入依赖。
- [不自动重试，写操作失败需要调用方自己处理] → 明确记录为设计取舍；重试属于行为变更，将来单独提变更。

## Migration Plan

1. **配置层**：`config/env.go` 新增 `SupabaseConfig`、`AppConfig` 字段、`setConfigValue` 分支、`setDefaults` 兜底；`.env.example` 补占位项。
2. **客户端**：新增 `internal/supabase/client.go`（`New` + 包级 `Get()` + 5 个方法 + 白名单校验）。
3. **测试**：新增 `internal/supabase/client_test.go`（httptest 单测 + env 门控集成测试）。
4. **文档**：`chaos-go/CONFIG.md` 补 Supabase 配置项与「新式 key 只放 `apikey` 头」「secret vs publishable」注意事项。
5. **验证**：`cd chaos-go && go build ./...`；`go test ./...`（全仓）；设置 `SUPABASE_URL` / `SUPABASE_SECRET_KEY` / `SUPABASE_DEMO_TABLE` 后跑一次集成测试打通真实链路。
6. **回滚**：删除 `internal/supabase` 包、移除 `SupabaseConfig` 相关代码与 `.env` 条目即可。本次无路由变更、无 GORM 模型变更、无数据库 schema 变更、无前端变更，对既有功能零影响。

## Open Questions

- 用于验证的具体表名与列结构待用户从 Supabase Dashboard 确认（通过 `SUPABASE_TABLES` / `SUPABASE_DEMO_TABLE` 注入，不影响本设计的任何决策与任务拆分）。
- 未来若要把云端能力暴露给 `chaos-ui`，是新增 `/api/supabase/*` 路由组还是并入既有模块的 handler，留待届时独立提案。
