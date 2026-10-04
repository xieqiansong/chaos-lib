# chaos-go

`chaos` 项目的 Go 后端服务（基于 gin）。提供任务计划、文件连接、SSH 端口转发、MQTT 多节点同步、定时任务、接口访问日志、项目管理等能力。

## 目录结构

目标布局见 [`Go Web项目结构.md`](./Go%20Web项目结构.md)。

```
chaos-go/
├── api/
│   └── openapi.yaml          # OpenAPI 定义
├── cmd/
│   ├── server/
│   │   └── main.go           # HTTP 服务入口
│   ├── tcp_over_websockets/  # 辅助工具：TCP over WebSocket
│   └── test/                 # 辅助命令行工具
├── configs/
│   ├── config.example.yaml   # 配置模板（入库）
│   └── config.yaml           # 真实配置（含密钥，被 .gitignore 忽略）
├── deployments/
│   ├── Dockerfile
│   └── docker-compose.yml    # Docker 部署
├── internal/                 # 私有代码，按「框架 / 平台 / 业务」分层
│   ├── framework/            # 框架与基础设施（不依赖任何业务包）
│   │   ├── resp/             # 统一响应信封
│   │   ├── pagination/       # 分页
│   │   ├── httpx/            # HTTP 工具
│   │   ├── routehub/         # 路由登记中心
│   │   ├── router/           # 路由装配
│   │   ├── config/           # 配置加载（config.go / loader.go / testing_hook.go）
│   │   ├── apilog/           # 接口访问日志
│   │   ├── datacache/        # 数据缓存
│   │   ├── memcache/         # 内存缓存
│   │   ├── dbmonitor/        # 数据库监控
│   │   ├── scheduler/        # 调度器
│   │   └── crud/             # 通用 CRUD
│   ├── platform/             # 外部系统集成适配器（无业务概念）
│   │   ├── deepseek/         # DeepSeek 集成
│   │   ├── supabase/         # Supabase 集成
│   │   ├── mqttsync/         # MQTT 多节点同步
│   │   ├── stunsync/         # STUN 同步
│   │   └── stunpf/           # STUN 端口转发
│   ├── domain/               # 业务领域模块
│   │   ├── taskplan/         # 任务计划
│   │   ├── project/          # 项目管理
│   │   ├── portfwd/          # SSH 端口转发
│   │   ├── filelink/         # 文件连接
│   │   ├── quickedit/        # 快速编辑
│   │   ├── note/             # 笔记
│   │   ├── proxy/            # 代理
│   │   ├── cronjob/          # 定时任务
│   │   ├── envvar/           # 环境变量管理
│   │   ├── sdk/              # 内部 SDK
│   │   ├── standarddata/     # 标准数据
│   │   └── systemjob/        # 系统任务
│   └── app/                  # 应用装配根（composition root）
├── migrations/               # 数据库 schema（原 sql/）
│   ├── chaos_postgres_schema.sql
│   ├── chaos_postgres_update.sql
│   └── supabase_schema.sql
├── pkg/
│   └── tools/                # 可被外部复用的工具库
├── scripts/                  # 脚本
├── test/
│   └── integration/          # 集成测试
├── web/
│   ├── static/               # 静态资源
│   └── templates/            # 模板
├── CONFIG.md
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 配置

配置加载与字段说明见 [`CONFIG.md`](./CONFIG.md)。敏感配置写入本地 `configs/config.yaml`（已被 `.gitignore` 忽略），勿入库。

## 构建与运行

```bash
make build   # 编译到 bin/server
make run     # 直接运行
make test    # 跑测试
```
