```text
myapp/
├── cmd/
│   └── server/
│       └── main.go              # 程序入口：加载配置、初始化依赖、启动 HTTP Server
├── internal/                    # 私有代码，只允许本模块导入
│   ├── config/                  # 配置结构体与加载逻辑
│   ├── router/                  # 路由注册
│   ├── middleware/              # 认证、日志、CORS、Recover 等中间件
│   ├── handler/                 # HTTP Handler / Controller
│   ├── service/                 # 业务逻辑
│   ├── repository/              # 数据访问层
│   │   └── mysql/
│   ├── model/                   # 实体、DTO、领域模型
│   └── app/                     # 应用装配、依赖注入
├── pkg/                         # 可被外部导入的公共库，谨慎使用
├── api/
│   └── openapi.yaml             # OpenAPI/Swagger 定义
├── web/
│   ├── static/                  # 静态资源：js/css/img
│   └── templates/               # HTML 模板
├── configs/
│   └── config.example.yaml      # 配置示例，不提交敏感配置
├── migrations/                  # 数据库迁移 SQL
├── scripts/                     # 构建、部署、生成代码脚本
├── deployments/
│   ├── Dockerfile
│   └── docker-compose.yml
├── test/
│   └── integration/             # 集成测试
├── go.mod
├── go.sum
├── Makefile
└── README.md
```