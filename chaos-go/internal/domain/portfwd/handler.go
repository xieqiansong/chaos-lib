package portfwd

import (
	"net/http"

	"chaos-go/internal/framework/crud"
	"chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/httpx"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
// 存量 /api 路由与新的 /api/v1 动作路由并存（双轨迁移）。
func init() {
	routehub.Register("portFwd", Register)
	routehub.RegisterV1("portFwd", RegisterV1)
}

// ruleErrRules 转发启停的领域错误 → HTTP 状态码映射表（未命中回落 500）。
var ruleErrRules = []httpx.ErrRule{
	{Err: ErrRuleNotFound, Status: http.StatusNotFound, Msg: "端口转发不存在"},
	{Err: ErrAlreadyRunning, Status: http.StatusBadRequest},
	{Err: ErrNotRunning, Status: http.StatusBadRequest},
	{Err: ErrConnRequired, Status: http.StatusBadRequest},
	{Err: ErrInvalidRule, Status: http.StatusBadRequest},
}

// connErrRules SSH 连接测试的领域错误映射表（未命中回落 400：连不上多为参数 / 凭据问题）。
var connErrRules = []httpx.ErrRule{
	{Err: ErrConnNotFound, Status: http.StatusNotFound, Msg: "SSH 连接不存在"},
}

// sshOpts / pfOpts / pfToggle 两个资源的 CRUD 与启停选项，存量 /api 与 v1 动作路由共用。
var sshOpts = crud.Opts[SshConnection]{
	Searchable:   []string{"name", "host", "username", "remark"},
	Sortable:     []string{"id", "name", "host", "username"},
	ToResponse:   sshConnsToResponse,
	BeforeCreate: ValidateSshConnForCreate,
	AfterUpdate:  ValidateSshConnForUpdate,
	AfterDelete:  GuardSshConnDeletion,
}
var pfOpts = crud.Opts[PortForwarding]{
	Searchable:   []string{"name", "direction", "target_host", "remark"},
	Sortable:     []string{"id", "name", "direction", "port", "target_port"},
	Protected:    []string{"Status"},
	ToResponse:   PortForwardingsToResponse,
	BeforeCreate: PreparePortForwardForCreate,
	AfterUpdate:  GuardPortForwardUpdate,
	AfterDelete:  StopPortForwardOnDelete,
}
var pfToggle = &crud.ToggleOpts{
	Setter:   func(id int, status bool) (any, error) { return SetForwardStatus(id, status) },
	ErrRules: ruleErrRules,
}

// Register 把 SSH 端口转发两个资源挂载到给定路由组：纯 CRUD 交给 crud（回调只做转发），扩展接口自实现。
// Status 标记为受保护字段：仅经专用启停路由改写，通用 PATCH 无法绕过。
func Register(rg *gin.RouterGroup) {
	// SSH 连接信息：凭据仅后端使用，响应一律脱敏；测试接口自实现。
	ssh := crud.Register[SshConnection](rg, "ssh-conns", sshOpts)
	ssh.POST("/:id/test", testSshConnection)

	// 端口转发规则：启停带隧道副作用，走基线的通用启停路由，本包只提供动作与错误表。
	pf := crud.Register[PortForwarding](rg, "port-forwards", pfOpts)
	crud.RegisterToggle(pf, *pfToggle)
}

// RegisterV1 以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// 两个资源的标准 CRUD + port-forwards 的 status 由通用 crud 生成；
// ssh-conns 的 test 自定义动作一并迁为 POST 动作。
func RegisterV1(rg *gin.RouterGroup) {
	ssh := crud.RegisterActions[SshConnection](rg, "ssh-conns", sshOpts, nil)
	ssh.POST("/test", testSshV1)
	crud.RegisterActions[PortForwarding](rg, "port-forwards", pfOpts, pfToggle)
}

// ── 自定义路由 ──────────────────────────────────────────────────

// testSshConnection 测试 SSH 连接可达性与凭据有效性（POST /ssh-conns/:id/test）。
func testSshConnection(c *gin.Context) {
	id, ok := httpx.ParseID(c)
	if !ok {
		return
	}
	doTestSsh(c, id)
}

// testSshV1 是 testSshConnection 的「POST + Action」版（POST /api/v1/ssh-conns/test，body {id}）。
func testSshV1(c *gin.Context) {
	var req struct {
		ID int `json:"id"`
	}
	if err := envelope.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	doTestSsh(c, req.ID)
}

func doTestSsh(c *gin.Context, id int) {
	result, err := TestSshConn(id)
	if err != nil {
		httpx.MapError(c, err, connErrRules, http.StatusBadRequest)
		return
	}
	renv.Success(c, result)
}
