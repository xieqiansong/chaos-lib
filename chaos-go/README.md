# chaos-go

`chaos` 项目的 Go 后端服务：自研 Web 框架（`net/http` 封装，**不依赖 gin**）+ GORM，提供任务计划、文件连接、SSH 端口转发、MQTT 多节点同步、定时任务、接口访问日志、项目管理等能力。

> 模块清单、路由与 API 前缀以 [`docs/MODULE_INDEX.md`](../docs/MODULE_INDEX.md) 为唯一事实源；架构机制见 [`docs/architecture.md`](../docs/architecture.md)；硬性规则见 [`AGENTS.md`](../AGENTS.md)。本文件只列后端入口、顶层目录与构建命令，不重复维护模块清单。

## 目录结构

```
chaos-go/
├── cmd/                    # 可执行入口：server（HTTP）、migrate，以及 natter / tcp_over_websockets 等辅助工具
├── internal/
│   ├── app/                # 组合根：config → DB → 插件 Build → 迁移 → 启动钩子 → 路由
│   ├── domain/             # 16 个业务插件（handler / service / repository / model / plugin）
│   ├── framework/          # 自研 Web 框架与跨切面基件（16 个，其中 2 个注册为插件）
│   └── platform/          # 5 个外部集成（其中 3 个注册为插件）
├── pkg/tools/              # 跨平台宿主交互工具（非业务）
├── configs/                # config.example.yaml（模板 / 字段权威）+ config.yaml（运行时，已忽略）
├── migrations/             # chaos_postgres_schema.sql（冻结快照，禁止修改）/ _seed / _update / supabase_schema
├── deployments/            # Dockerfile + docker-compose.yml
├── api/                    # openapi.yaml（占位，冻结）
├── CONFIG.md               # 配置字段说明
├── Makefile
└── go.mod / go.sum         # module chaos-go，Go 1.27
```

> 模块名与落点见 `docs/MODULE_INDEX.md`。`config.GetDB()` 只允许出现在 `repository.go`；新增模型登记到 `plugin.go` 的 `Models`，由 `config.AutoMigrate` 迁移，详见 `AGENTS.md`。前端静态资源不放在本工程内。

## 配置

配置加载与字段说明见 [`CONFIG.md`](./CONFIG.md)。敏感配置写入本地 `configs/config.yaml`（已被 `.gitignore` 忽略），勿入库。

## 构建与运行

```bash
make build   # 编译到 bin/server
make run     # 直接运行
make test    # 跑测试
```

提交前须 `go build ./...` 无错；改动后端逻辑后跑 `go test ./...`（见 `AGENTS.md`）。前端产物由根仓库 `scripts/chaos.deploy.ps1` 构建并拷到 exe 同目录 `ui/`，运行时由 `CHAOS_UI_DIR` 或 `./ui` 解析。
