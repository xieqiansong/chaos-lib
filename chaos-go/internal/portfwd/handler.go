package portfwd

import (
	"fmt"
	"log/slog"
	"strconv"

	"chaos-go/internal/crud"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// ── 自定义路由：启停状态 ──────────────────────────────────────────

// UpdatePortForwardingStatus 启停单条转发规则。
// local/remote 经关联的 SSH 连接建立隧道；direct 直接在本机监听并直连目标。
func UpdatePortForwardingStatus(c *gin.Context) {
	id := c.Param("id")
	rule, err := FindPortForwardingByID(id)
	if err != nil {
		renv.Error(c, 404, "端口转发不存在")
		return
	}
	if err := rule.normalize(); err != nil {
		renv.Error(c, 400, err.Error())
		return
	}
	var req struct {
		Status bool
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		renv.Error(c, 400, err.Error())
		return
	}
	running, _ := GlobalPortForwarder.Status(rule.ID)
	if req.Status {
		if running {
			renv.Error(c, 400, "该端口转发已启动")
			return
		}
		// 直接转发不经 SSH 隧道，无需加载 SSH 连接；其余方向必须存在对应连接。
		var conn *SshConnection
		if rule.Direction != DirectionDirect {
			loaded, lerr := FindSshConnByID(strconv.Itoa(rule.SshConnectionId))
			if lerr != nil {
				renv.Error(c, 400, "SSH 连接不存在，无法启动")
				return
			}
			conn = loaded
		}
		if err := GlobalPortForwarder.AddForward(rule, conn); err != nil {
			renv.Error(c, 500, "启动端口转发失败: "+err.Error())
			return
		}
		rule.Status = true
	} else {
		if !running {
			renv.Error(c, 400, "该端口转发未启动")
			return
		}
		if err := GlobalPortForwarder.RemoveForward(rule.ID); err != nil {
			renv.Error(c, 500, "停止端口转发失败: "+err.Error())
			return
		}
		rule.Status = false
	}
	if err := SavePortForwarding(rule); err != nil {
		renv.Error(c, 500, "更新状态失败: "+err.Error())
		return
	}
	renv.Success(c, rule.toResponse())
}

// TestSshConnection 测试 SSH 连接可达性与凭据有效性（不启动端口转发）。
func TestSshConnection(c *gin.Context) {
	id := c.Param("id")
	conn, err := FindSshConnByID(id)
	if err != nil {
		renv.Error(c, 404, "SSH 连接不存在")
		return
	}
	client, err := conn.dial()
	if err != nil {
		slog.Warn("SSH 连接测试失败", "sshAddr", conn.sshAddr(), "username", conn.Username, "err", err)
		renv.Error(c, 400, "连接失败: "+err.Error())
		return
	}
	defer client.Close()
	renv.Success(c, gin.H{
		"sshAddr":       conn.sshAddr(),
		"serverVersion": string(client.ServerVersion()),
		"remoteAddr":    client.RemoteAddr().String(),
	})
}

// ── CRUD 回调 ─────────────────────────────────────────────────────

func beforeCreateSshConn(row *SshConnection) error {
	return row.validateForSave(true)
}

func afterUpdateSshConn(row *SshConnection) error {
	return row.validateForSave(false)
}

// afterDeleteSshConn 删除前拦截：若仍被任意转发规则引用则拒绝，并回滚软删除。
func afterDeleteSshConn(row *SshConnection) error {
	count, err := CountPortForwardingsByConn(row.ID)
	if err != nil {
		return fmt.Errorf("校验关联转发失败: %v", err)
	}
	if count > 0 {
		return fmt.Errorf("该 SSH 连接被 %d 条转发规则引用，请先删除相关规则", count)
	}
	return nil
}

// beforeCreatePortForward 创建前规范化并校验（名称留空自动生成）。
func beforeCreatePortForward(row *PortForwarding) error {
	pf := row
	pf.Status = false
	return pf.validate()
}

// afterUpdatePortForward 更新前校验；运行中的规则禁止任何修改，需先停止。
func afterUpdatePortForward(row *PortForwarding) error {
	pf := row
	if running, _ := GlobalPortForwarder.Status(pf.ID); running {
		return fmt.Errorf("该转发正在运行，请先停止后再修改")
	}
	return pf.validate()
}

// afterDeletePortForward 删除（软删）前若正在运行则先停掉隧道。
func afterDeletePortForward(row *PortForwarding) error {
	pf := row
	if running, _ := GlobalPortForwarder.Status(pf.ID); running {
		if err := GlobalPortForwarder.RemoveForward(pf.ID); err != nil {
			return fmt.Errorf("停止端口转发失败: %v", err)
		}
	}
	return nil
}

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("portFwd", Register)
}

// Register 把 SSH 端口转发两个资源挂载到给定路由组：纯 CRUD 交给 crud，扩展接口自实现。
// Status 标记为受保护字段：仅经专用启停路由改写，通用 PATCH 无法绕过。
func Register(rg *gin.RouterGroup) {
	// SSH 连接信息：凭据仅后端使用，响应一律脱敏；测试接口自实现。
	crud.Register[SshConnection](rg, "sshConns", crud.Opts[SshConnection]{
		Searchable:   []string{"name", "host", "username", "remark"},
		Sortable:     []string{"id", "name", "host", "username"},
		ToResponse:   sshConnsToResponse,
		BeforeCreate: beforeCreateSshConn,
		AfterUpdate:  afterUpdateSshConn,
		AfterDelete:  afterDeleteSshConn,
	})
	rg.Group("/sshConns").POST("/:id/test", TestSshConnection)

	// 端口转发规则
	crud.Register[PortForwarding](rg, "portForwards", crud.Opts[PortForwarding]{
		Searchable:   []string{"name", "direction", "target_host", "remark"},
		Sortable:     []string{"id", "name", "direction", "port", "target_port"},
		Protected:    []string{"Status"},
		ToResponse:   portForwardingsToResponse,
		BeforeCreate: beforeCreatePortForward,
		AfterUpdate:  afterUpdatePortForward,
		AfterDelete:  afterDeletePortForward,
	})
	rg.Group("/portForwards").PATCH("/:id/status", UpdatePortForwardingStatus)
}
