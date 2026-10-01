# chaos-go

`chaos` 项目的 Go 后端服务（基于 gin）。提供任务计划、文件连接、SSH 端口转发、MQTT 多节点同步、定时任务、接口访问日志、项目管理等能力。

## 目录结构

目标布局见 [`Go Web项目结构.md`](./Go%20Web项目结构.md)；当前主要目录：

- `cmd/server`：HTTP 服务入口（`main.go`）
- `cmd/tcp_over_websockets`、`cmd/test`：辅助命令行工具
- `internal/`：私有业务代码（按功能分包，如 `taskplan`、`filelink`、`portfwd`、`proxy` 等；配置在 `internal/config`，路由在 `internal/router`）
- `pkg/tools`：可被外部复用的工具库
- `migrations/`：数据库 schema（原 `sql/`）
- `configs/`：配置示例（不提交敏感信息）
- `deployments/`：Docker 部署
- `api/`：OpenAPI 定义

## 配置

配置加载与字段说明见 [`CONFIG.md`](./CONFIG.md)。敏感配置通过环境变量 / 本地 `config.yaml` 注入，勿入库。

## 构建与运行

```bash
make build   # 编译到 bin/server
make run     # 直接运行
make test    # 跑测试
```
