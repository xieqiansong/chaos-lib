package portfwd

import (
	"chaos-go/config"
	"fmt"
	"net"
	"strconv"
	"strings"

	"chaos-go/internal/crud"

	"github.com/gin-gonic/gin"
)

// ── 模型 ──────────────────────────────────────────────────────────

const (
	// DirectionLocal 本地转发（等价 ssh -L）：本机监听 → SSH 服务器侧解析目标
	DirectionLocal = "local"
	// DirectionRemote 远程转发（等价 ssh -R）：SSH 服务器侧监听 → 本机侧解析目标
	DirectionRemote = "remote"
	// DirectionDirect 直接转发：本机监听 → 直接（纯 TCP）拨向目标，不经 SSH 隧道
	DirectionDirect = "direct"

	// DefaultLocalBindAddress 本地转发缺省监听地址（监听全部网卡，与既有行为一致）
	DefaultLocalBindAddress = "0.0.0.0"
	// DefaultRemoteBindAddress 远程转发缺省监听地址（与 OpenSSH ssh -R 不带 bind_address 一致）
	DefaultRemoteBindAddress = "127.0.0.1"
	// DefaultDirectBindAddress 直接转发缺省监听地址（监听全部网卡）
	DefaultDirectBindAddress = "0.0.0.0"
)

// isLocalSideListen 该方向是否在本机监听（本地转发与直接转发都在本机侧监听）。
func isLocalSideListen(direction string) bool {
	return direction == DirectionLocal || direction == DirectionDirect
}

// PortForwarding 端口转发规则。
// 嵌入 crud.BaseModel 以获得统一主键 / 时间戳 / 软删除。
// Direction 决定「监听」发生在哪一侧：local 在本机监听，remote 在 SSH 服务器侧监听。
// Status 为「期望运行」标记（供重启恢复 / 自愈使用）；实际运行状态以内存为准（见 toResponse）。
type PortForwarding struct {
	crud.BaseModel
	Name            string
	Direction       string
	Port            int
	BindAddress     string
	TargetHost      string
	TargetPort      int
	SshConnectionId int
	Status          bool
	LastError       string
	Remark          string
}

func (PortForwarding) TableName() string {
	return "port_forwarding"
}

// PortForwardingResponse 对外 DTO；Status 以内存中实际运行状态为准。
type PortForwardingResponse struct {
	ID              int    `json:"ID"`
	Name            string `json:"Name"`
	Direction       string
	Port            int
	BindAddress     string
	TargetHost      string
	TargetPort      int
	SshConnectionId int
	Status          bool
	LastError       string
	Remark          string
}

// displayBindAddress 监听地址展示值：空值时按方向取默认（仅用于展示，运行期 normalize 另行兜底）。
func (pf *PortForwarding) displayBindAddress() string {
	if strings.TrimSpace(pf.BindAddress) != "" {
		return pf.BindAddress
	}
	switch pf.Direction {
	case DirectionRemote:
		return DefaultRemoteBindAddress
	case DirectionDirect:
		return DefaultDirectBindAddress
	default:
		return DefaultLocalBindAddress
	}
}

// normalize 补齐方向与监听地址：方向空值视为 local（兼容本次变更前的存量数据）。
func (pf *PortForwarding) normalize() error {
	switch pf.Direction {
	case "":
		pf.Direction = DirectionLocal
	case DirectionLocal, DirectionRemote, DirectionDirect:
	default:
		return fmt.Errorf("转发方向不合法，只能为 %s、%s 或 %s", DirectionLocal, DirectionRemote, DirectionDirect)
	}
	if strings.TrimSpace(pf.BindAddress) == "" {
		switch pf.Direction {
		case DirectionRemote:
			pf.BindAddress = DefaultRemoteBindAddress
		case DirectionDirect:
			pf.BindAddress = DefaultDirectBindAddress
		default:
			pf.BindAddress = DefaultLocalBindAddress
		}
	}
	pf.BindAddress = strings.TrimSpace(pf.BindAddress)
	return nil
}

// listenAddr 监听地址：local 为本机监听地址，remote 为 SSH 服务器侧监听地址。
func (pf *PortForwarding) listenAddr() string {
	return net.JoinHostPort(pf.BindAddress, strconv.Itoa(pf.Port))
}

// targetAddr 目标地址：local 由 SSH 服务器侧解析，remote 由本机侧解析。
func (pf *PortForwarding) targetAddr() string {
	return fmt.Sprintf("%s:%d", pf.TargetHost, pf.TargetPort)
}

func (pf *PortForwarding) directionLabel() string {
	switch pf.Direction {
	case DirectionRemote:
		return "远端"
	case DirectionDirect:
		return "直接"
	default:
		return "本地"
	}
}

func (pf *PortForwarding) toResponse() PortForwardingResponse {
	running, lastErr := GlobalPortForwarder.Status(pf.ID)
	return PortForwardingResponse{
		ID:              pf.ID,
		Name:            pf.Name,
		Direction:       pf.Direction,
		Port:            pf.Port,
		BindAddress:     pf.displayBindAddress(),
		TargetHost:      pf.TargetHost,
		TargetPort:      pf.TargetPort,
		SshConnectionId: pf.SshConnectionId,
		Status:          running,
		LastError:       lastErr,
		Remark:          pf.Remark,
	}
}

// portForwardingsToResponse 整批把 []*PortForwarding 转成响应 DTO（供 crud.ToResponse 调用）。
func portForwardingsToResponse(rows any) any {
	rules := rows.([]*PortForwarding)
	out := make([]PortForwardingResponse, 0, len(rules))
	for _, r := range rules {
		// 存量行可能没有 direction，读路径同样按 local 兜底
		_ = r.normalize()
		out = append(out, r.toResponse())
	}
	return out
}

// validate 校验规则字段；需关联已存在的 SSH 连接（direct 除外）。空名称自动生成。
func (pf *PortForwarding) validate() error {
	if err := pf.normalize(); err != nil {
		return err
	}
	if err := validatePort(pf.Port); err != nil {
		return fmt.Errorf("%s监听端口不合法: %v", pf.directionLabel(), err)
	}
	if strings.ContainsAny(pf.BindAddress, " \t/") {
		return fmt.Errorf("监听地址不合法: %s", pf.BindAddress)
	}
	if strings.TrimSpace(pf.TargetHost) == "" {
		return fmt.Errorf("目标主机不能为空")
	}
	if err := validatePort(pf.TargetPort); err != nil {
		return fmt.Errorf("目标端口不合法: %v", err)
	}
	// 直接转发不经 SSH 隧道，无需关联 SSH 连接；其余方向必须关联一条已存在的连接。
	if pf.Direction != DirectionDirect {
		if pf.SshConnectionId <= 0 {
			return fmt.Errorf("请选择 SSH 连接")
		}
		var conn SshConnection
		if result := config.GetDB().Where("is_deleted = ?", false).First(&conn, "id = ?", pf.SshConnectionId); result.Error != nil {
			return fmt.Errorf("SSH 连接不存在")
		}
	}
	if strings.TrimSpace(pf.Name) == "" {
		switch pf.Direction {
		case DirectionRemote:
			pf.Name = fmt.Sprintf("[R] %s → %s", pf.listenAddr(), pf.targetAddr())
		case DirectionDirect:
			pf.Name = fmt.Sprintf("[D] %s → %s", pf.listenAddr(), pf.targetAddr())
		default:
			pf.Name = fmt.Sprintf("[L] %s → %s", pf.listenAddr(), pf.targetAddr())
		}
	}
	return nil
}

// beforeCreatePortForward 创建前规范化并校验（名称留空自动生成）。
func beforeCreatePortForward(row any) error {
	pf := row.(*PortForwarding)
	pf.Status = false
	return pf.validate()
}

// afterUpdatePortForward 更新前校验；运行中的规则禁止任何修改，需先停止。
func afterUpdatePortForward(row any) error {
	pf := row.(*PortForwarding)
	if running, _ := GlobalPortForwarder.Status(pf.ID); running {
		return fmt.Errorf("该转发正在运行，请先停止后再修改")
	}
	return pf.validate()
}

// afterDeletePortForward 删除（软删）前若正在运行则先停掉隧道。
func afterDeletePortForward(row any) error {
	pf := row.(*PortForwarding)
	if running, _ := GlobalPortForwarder.Status(pf.ID); running {
		if err := GlobalPortForwarder.RemoveForward(pf.ID); err != nil {
			return fmt.Errorf("停止端口转发失败: %v", err)
		}
	}
	return nil
}

// ── 自定义路由：启停状态 ──────────────────────────────────────────

// UpdatePortForwardingStatus 启停单条转发规则。
// local/remote 经关联的 SSH 连接建立隧道；direct 直接在本机监听并直连目标。
func UpdatePortForwardingStatus(c *gin.Context) {
	id := c.Param("id")
	var rule PortForwarding
	if result := config.GetDB().Where("is_deleted = ?", false).First(&rule, "id = ?", id); result.Error != nil {
		c.JSON(404, gin.H{"error": "端口转发不存在"})
		return
	}
	if err := rule.normalize(); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var req struct {
		Status bool
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	running, _ := GlobalPortForwarder.Status(rule.ID)
	if req.Status {
		if running {
			c.JSON(400, gin.H{"error": "该端口转发已启动"})
			return
		}
		// 直接转发不经 SSH 隧道，无需加载 SSH 连接；其余方向必须存在对应连接。
		var conn *SshConnection
		if rule.Direction != DirectionDirect {
			var loaded SshConnection
			if result := config.GetDB().Where("is_deleted = ?", false).First(&loaded, "id = ?", rule.SshConnectionId); result.Error != nil {
				c.JSON(400, gin.H{"error": "SSH 连接不存在，无法启动"})
				return
			}
			conn = &loaded
		}
		if err := GlobalPortForwarder.AddForward(&rule, conn); err != nil {
			c.JSON(500, gin.H{"error": "启动端口转发失败: " + err.Error()})
			return
		}
		rule.Status = true
	} else {
		if !running {
			c.JSON(400, gin.H{"error": "该端口转发未启动"})
			return
		}
		if err := GlobalPortForwarder.RemoveForward(rule.ID); err != nil {
			c.JSON(500, gin.H{"error": "停止端口转发失败: " + err.Error()})
			return
		}
		rule.Status = false
	}
	if result := config.GetDB().Save(&rule); result.Error != nil {
		c.JSON(500, gin.H{"error": "更新状态失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "状态更新成功", "data": rule.toResponse()})
}

// Register 把 SSH 端口转发两个资源挂载到给定路由组：纯 CRUD 交给 crud，扩展接口自实现。
// Status 标记为受保护字段：仅经专用启停路由改写，通用 PATCH 无法绕过。
func Register(rg *gin.RouterGroup) {
	// SSH 连接信息：凭据仅后端使用，响应一律脱敏；测试接口自实现。
	crud.Register(rg, "sshConns", &SshConnection{}, crud.Opts{
		Searchable:   []string{"name", "host", "username", "remark"},
		Sortable:     []string{"id", "name", "host", "username"},
		ToResponse:   sshConnsToResponse,
		BeforeCreate: beforeCreateSshConn,
		AfterUpdate:  afterUpdateSshConn,
		AfterDelete:  afterDeleteSshConn,
	})
	rg.Group("/sshConns").POST("/:id/test", TestSshConnection)

	// 端口转发规则
	crud.Register(rg, "portForwards", &PortForwarding{}, crud.Opts{
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
