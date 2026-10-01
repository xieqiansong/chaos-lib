package portfwd

import (
	"errors"
	"net/http"
	"strconv"

	"chaos-go/internal/crud"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("portFwd", Register)
}

// Register 把 SSH 端口转发两个资源挂载到给定路由组：纯 CRUD 交给 crud（回调只做转发），扩展接口自实现。
// Status 标记为受保护字段：仅经专用启停路由改写，通用 PATCH 无法绕过。
func Register(rg *gin.RouterGroup) {
	// SSH 连接信息：凭据仅后端使用，响应一律脱敏；测试接口自实现。
	crud.Register[SshConnection](rg, "sshConns", crud.Opts[SshConnection]{
		Searchable:   []string{"name", "host", "username", "remark"},
		Sortable:     []string{"id", "name", "host", "username"},
		ToResponse:   sshConnsToResponse,
		BeforeCreate: ValidateSshConnForCreate,
		AfterUpdate:  ValidateSshConnForUpdate,
		AfterDelete:  GuardSshConnDeletion,
	})
	rg.Group("/sshConns").POST("/:id/test", testSshConnection)

	// 端口转发规则
	crud.Register[PortForwarding](rg, "portForwards", crud.Opts[PortForwarding]{
		Searchable:   []string{"name", "direction", "target_host", "remark"},
		Sortable:     []string{"id", "name", "direction", "port", "target_port"},
		Protected:    []string{"Status"},
		ToResponse:   PortForwardingsToResponse,
		BeforeCreate: PreparePortForwardForCreate,
		AfterUpdate:  GuardPortForwardUpdate,
		AfterDelete:  StopPortForwardOnDelete,
	})
	rg.Group("/portForwards").PATCH("/:id/status", updatePortForwardingStatus)
}

// ── 自定义路由 ──────────────────────────────────────────────────

// testSshConnection 测试 SSH 连接可达性与凭据有效性（POST /sshConns/:id/test）。
func testSshConnection(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	result, err := TestSshConn(id)
	if err != nil {
		if errors.Is(err, ErrConnNotFound) {
			renv.Error(c, http.StatusNotFound, "SSH 连接不存在")
			return
		}
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	renv.Success(c, result)
}

// updatePortForwardingStatus 启停单条转发规则（PATCH /portForwards/:id/status，body {status:bool}）。
func updatePortForwardingStatus(c *gin.Context) {
	var req struct {
		Status bool
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return
	}
	resp, err := SetForwardStatus(id, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, ErrRuleNotFound):
			renv.Error(c, http.StatusNotFound, "端口转发不存在")
		case errors.Is(err, ErrAlreadyRunning), errors.Is(err, ErrNotRunning),
			errors.Is(err, ErrConnRequired), errors.Is(err, ErrInvalidRule):
			renv.Error(c, http.StatusBadRequest, err.Error())
		default:
			renv.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	renv.Success(c, resp)
}
