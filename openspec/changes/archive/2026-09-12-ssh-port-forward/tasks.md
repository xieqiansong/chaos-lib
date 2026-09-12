## 1. 数据模型与迁移

- [x] 1.1 在 `chaos-go/internal/portfwd/` 新增 `SshConnection` 模型（`Id`/`Name`/`Host`/`Port`/`Username`/`AuthType`/`Password`/`PrivateKey`/`Passphrase`/`Remark`，其中 `AuthType` 取 `password` 或 `key`），`TableName()` 返回 `ssh_connections`；验证方式：`go build ./...` 通过且模型无 `gorm:"column:..."` 标签 → 见 `internal/portfwd/sshconn.go`
- [x] 1.2 扩展 `internal/portfwd/portfwd.go` 的 `PortForwarding` 模型：新增 `SshConnectionId`、`Remark`，保留 `Name`/`Port`/`TargetHost`/`TargetPort`/`Status`；验证方式：`go build ./...` 通过
- [x] 1.3 删除 `chaos-go/models/port_forwarding.go` 中重复且未使用的模型定义（保留 `internal/portfwd` 内模型，遵循「模型 + handler 同包」）；验证方式：全局搜索 `models.PortForwarding` 无引用，`go build ./...` 通过
- [x] 1.4 在 `cmd/server/main.go` 的 `AutoMigrate()` 登记 `SshConnection`（`PortForwarding` 已在登记列表中）；验证方式：启动服务后 SQLite 库中出现 `ssh_connections` 表与 `port_forwarding` 新列 → 冒烟测试中 `POST/GET /api/sshConns` 与引用 `SshConnectionId` 的规则均读写成功，证明表与列已建成
- [x] 1.5 在 `chaos-go/sql/chaos_postgres_update.sql` 末尾追加本条变更的增量 DDL（`CREATE TABLE ssh_connections` + `ALTER TABLE port_forwarding ADD COLUMN`），注明日期与用途、字段与代码模型一致；验证方式：人工比对模型字段与 DDL 列一一对应（**未修改 `chaos_postgres_schema.sql`**）
- [x] 1.6 将 `golang.org/x/crypto` 由 indirect 提升为直接依赖（`go mod tidy`）；验证方式：`go.mod` 中 `golang.org/x/crypto v0.48.0` 已位于直接 require 块，`go build ./...` 通过

## 2. SSH 隧道转发引擎

- [x] 2.1 在 `internal/portfwd/` 新增 SSH 客户端构建逻辑：支持密码认证与私钥认证（可选 passphrase，`ssh.ParsePrivateKeyWithPassphrase`），连接超时可控，Host Key 采用 `ssh.InsecureIgnoreHostKey` 并在代码注释中写明权衡；验证方式：已实测——密码认证走到握手阶段（`attempted methods [none password]`），私钥路径返回可读的 `解析私钥失败` 错误
- [x] 2.2 改造 `PortForwarder.AddForward`：先认证建立 `*ssh.Client`，再 `net.Listen` 本地端口；每客户端连接通过 `sshClient.Dial("tcp", targetAddr)` 建 `direct-tcpip` 通道后与本地连接做双向 `io.Copy`（复用 `ForwardTask` 的并发连接管理）；验证方式：实现完成，数据面需真实 SSH 凭据验证（见 7.7）
- [x] 2.3 `AddForward` 对「本地端口已被占用」「同一规则重复启动」返回明确错误（不再静默 `return nil`）；验证方式：实现完成，分支需真实 SSH 凭据验证（见 7.8）
- [x] 2.4 实现保活与断线重连：定时 `sshClient.SendRequest("keepalive@openssh.com", true, nil)`，仅对网络类错误重连（重建 `*ssh.Client`），认证失败不重连；验证方式：实现完成，需真实 SSH 凭据验证断网重连（见 7.9）
- [x] 2.5 `RemoveForward` / `StopAll` 顺序关闭 listener → 活动连接 → `ssh.Client`，释放本地端口；验证方式：实现完成，需真实 SSH 凭据验证释放端口（见 7.9）
- [x] 2.6 提供运行状态查询（哪些规则在运行、最近一次错误），供列表接口判定「内存实际运行状态」；验证方式：`GET /api/portForwards` 返回 `Status` / `LastError`，实测启动失败后错误可见
- [x] 2.7 服务启动时将所有 `port_forwarding.Status` 归零（进程重启后无隧道，回归未运行）；验证方式：`ResetStatusOnBoot()` 已在 `main.go` 调用，重启后数据仍可读（见 7.13）

## 3. REST API 与路由

- [x] 3.1 实现 SSH 连接 CRUD handler：`GET/POST /api/sshConns`、`PATCH/DELETE /api/sshConns/:id`；列表与详情返回脱敏 DTO（不含 `Password`/`PrivateKey`/`Passphrase`，仅给 `HasPassword` 等标记）；`PATCH` 凭据字段为空表示保持原值；删除前校验是否被规则引用，被引用则拒绝；验证方式：实测列表响应不含凭据明文，仅改名称后凭据保留，切换认证方式缺凭据被拒
- [x] 3.2 实现连通性测试 handler：`POST /api/sshConns/:id/test`，不启动转发即完成认证并返回服务器标识或失败原因；验证方式：实测错误凭据返回可读失败原因，且日志中无凭据
- [x] 3.3 实现转发规则 CRUD handler：`GET/POST /api/portForwards`、`PATCH/DELETE /api/portForwards/:id`；校验关联 SSH 连接存在、端口范围、目标主机非空；运行中拒绝修改（提示先停止）；删除运行中规则先停止再删除；验证方式：实测非法端口 / 不存在的 SSH 连接均返回 400，正常创建与删除 200
- [x] 3.4 实现启停 handler：`PATCH /api/portForwards/:id/status`（`{status: bool}`），启动/停止后落库并返回最新状态；重复启停返回 4xx；验证方式：实测启动失败（凭据错误）返回 500 且规则保持未运行，停止未运行规则返回 400
- [x] 3.5 在 `routes.SetupRouter` 注册上述两组路由（前缀 `/api`，风格对齐现有 `fileLinks`/`sdks`）；验证方式：`go build ./...` 通过，`/api/portForwards`、`/api/sshConns` 实测返回 JSON
- [ ] 3.6 全链路自测：以一条真实 SSH 连接建规则 → 启动 → 本地端口访问到远端目标 → 停止 → 删除；验证方式：`curl` 或本地端口连通性验证通过，日志中无凭据明文 → **API 侧已全部实测通过；隧道数据面待用户真实 SSH 环境验证（见 7.7）**

## 4. 前端页面（chaos-ui）

- [x] 4.1 `chaos-ui/src/utils/api.ts` 追加 SSH 连接与转发规则的类型定义与封装函数（`getSshConns` / `createSshConn` / `updateSshConn` / `deleteSshConn` / `testSshConn` / `getPortForwards` / `createPortForward` / `updatePortForward` / `deletePortForward` / `updatePortForwardStatus`）；验证方式：`pnpm build` 通过
- [x] 4.2 新增 `chaos-ui/src/views/PortForward.vue`：SSH 连接区（表格 + 新建/编辑弹窗 + 测试按钮 + 删除）与转发规则区（表格 + 新建/编辑弹窗 + 启停开关 + 运行状态与最近错误列）；编辑时凭据输入框不回显、留空即不修改；验证方式：`pnpm build` 通过并产出 `PortForward-*.js`
- [x] 4.3 `chaos-ui/src/router/index.ts` 追加路由 `{path:'/portForward', name:'portForward', component: () => import('@/views/PortForward.vue'), meta:{title:'端口转发', icon:'Connection'}}`；验证方式：侧边菜单由路由表自动生成，构建产物包含该页面

## 5. 构建与整体验收

- [x] 5.1 `cd chaos-go && go build ./...` 无编译错误；验证方式：命令输出为空
- [x] 5.2 `cd chaos-ui && pnpm build` 通过，并把 `chaos-ui/dist` 同步到 `chaos-go/cmd/server/web`（该目录已 gitignore），后端二进制可重新构建嵌入；验证方式：`go build .\cmd\server\main.go` 成功，embed 目录 153 个文件
- [ ] 5.3 按第 7 节「前端手动验证清单」逐项人工验收并记录结果；验证方式：清单全部勾选或记录未通过项 → **待用户执行**

## 6. 后端测试（决策：本次不添加）

- [x] 6.1 用户已确认**本次不补充 `_test.go`**，仅按第 7 节做人工验收。原因：SSH 隧道需真实 sshd 或可注入的最小接口才能有效覆盖，建设成本高于本次收益；功能验收以「本地端口经隧道访问远端目标服务」为准。
- [x] 6.2 记录：本节为决策留痕，归档时保留；后续若需回归防护，再单独提一个补测试的 change。

## 7. 前端手动验证清单（用户人工验收）

- [ ] 7.1 打开「端口转发」页面，SSH 连接列表与转发规则列表均正常加载，无控制台报错
- [ ] 7.2 新建一条「密码」认证的 SSH 连接并保存，列表出现该连接，凭据列不显示明文
- [ ] 7.3 新建一条「私钥」认证的 SSH 连接（粘贴私钥，可选口令）并保存成功
- [ ] 7.4 点击「测试」按钮：主机/凭据正确时提示成功；填错主机或密码时提示可读失败原因
- [ ] 7.5 编辑一条已有连接只改名称并保存，凭据输入框为空且保存后原凭据仍可用（测试仍通过）
- [ ] 7.6 新建转发规则并绑定 SSH 连接（本地端口、目标主机、目标端口），保存成功且初始为未运行
- [ ] 7.7 点击启动：状态变为运行中；用本地客户端连接本地端口可访问仅 SSH 服务器内网可达的目标服务
- [ ] 7.8 启动一条本地端口已被占用的规则，页面提示端口占用且规则保持未运行
- [ ] 7.9 点击停止：状态变为未运行，本地端口释放（再次启动可成功）
- [ ] 7.10 尝试修改运行中的规则，页面提示需先停止
- [ ] 7.11 尝试删除一条被规则引用的 SSH 连接，页面提示被引用并拒绝
- [ ] 7.12 删除一条运行中的转发规则，规则先被停止再被删除
- [ ] 7.13 刷新页面 / 重启后端后，所有规则状态为未运行，连接与规则数据仍在
- [ ] 7.14 查看后端日志：整个操作过程中无 SSH 密码 / 私钥 / 口令明文

## 8. 实施与本地验证记录（AI）

- 代码落点：`chaos-go/internal/portfwd/{sshconn.go,portfwd.go,forwarder.go}`、`chaos-go/routes/routes.go`、`chaos-go/cmd/server/main.go`、`chaos-go/sql/chaos_postgres_update.sql`、`chaos-ui/src/{views/PortForward.vue,utils/api.ts,router/index.ts,App.vue}`
- 本地冒烟（临时目录 + 临时 SQLite + 端口 18080/18081，已清理，未触碰真实库与服务）：
  - `POST/GET /api/sshConns`：凭据不回显（`HasPassword`/`HasPrivateKey` 布尔替代），端口缺省落库为 22
  - `PATCH /api/sshConns/:id` 仅改名称 → 凭据保留；切换 `key` 但未提供私钥 → 400；提供伪造私钥 → 测试接口返回 `解析私钥失败`（无内容泄露）
  - `POST /api/sshConns/:id/test`：错误凭据返回可读原因，日志仅有 `sshAddr`/`username`/错误摘要，**无凭据明文**
  - `POST /api/portForwards`：非法端口 / 不存在的 SSH 连接 → 400；正常创建 → 200 且自动命名、初始未运行
  - `PATCH /api/portForwards/:id/status`：启动因 SSH 认证失败 → 500 且规则保持未运行；停止未运行规则 → 400
  - `DELETE /api/sshConns/:id`（被规则引用）→ 400 并提示被引用条数；先删规则后可删连接
  - 重启进程后连接与规则数据仍在（SQLite 持久化 OK）
- 待用户在真实 SSH 环境验证：隧道数据面（7.7）、端口占用分支（7.8）、停止释放端口（7.9）、断网重连（7.9 相关）
