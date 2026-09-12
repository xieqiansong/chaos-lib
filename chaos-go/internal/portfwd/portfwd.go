package portfwd

import (
	"chaos-go/config"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ── 模型 ──────────────────────────────────────────────────────────

const (
	// DirectionLocal 本地转发（等价 ssh -L）：本机监听 → SSH 服务器侧解析目标
	DirectionLocal = "local"
	// DirectionRemote 远程转发（等价 ssh -R）：SSH 服务器侧监听 → 本机侧解析目标
	DirectionRemote = "remote"

	// DefaultLocalBindAddress 本地转发缺省监听地址（监听全部网卡，与既有行为一致）
	DefaultLocalBindAddress = "0.0.0.0"
	// DefaultRemoteBindAddress 远程转发缺省监听地址（与 OpenSSH ssh -R 不带 bind_address 一致）
	DefaultRemoteBindAddress = "127.0.0.1"
)

// PortForwarding 端口转发规则。
// Direction 决定「监听」发生在哪一侧：local 在本机监听，remote 在 SSH 服务器侧监听。
type PortForwarding struct {
	Id              int `gorm:"primaryKey"`
	Name            string
	Direction       string
	Port            int
	BindAddress     string
	TargetHost      string
	TargetPort      int
	SshConnectionId int
	Status          bool
	Remark          string
}

func (PortForwarding) TableName() string {
	return "port_forwarding"
}

// PortForwardingResponse 对外 DTO；Status 以内存中实际运行状态为准。
type PortForwardingResponse struct {
	Id              int
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

// normalize 补齐方向与监听地址：方向空值视为 local（兼容本次变更前的存量数据）。
func (pf *PortForwarding) normalize() error {
	switch pf.Direction {
	case "":
		pf.Direction = DirectionLocal
	case DirectionLocal, DirectionRemote:
	default:
		return fmt.Errorf("转发方向不合法，只能为 %s 或 %s", DirectionLocal, DirectionRemote)
	}
	if strings.TrimSpace(pf.BindAddress) == "" {
		if pf.Direction == DirectionRemote {
			pf.BindAddress = DefaultRemoteBindAddress
		} else {
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
	if pf.Direction == DirectionRemote {
		return "远端"
	}
	return "本地"
}

func (pf *PortForwarding) toResponse() PortForwardingResponse {
	running, lastErr := GlobalPortForwarder.Status(pf.Id)
	return PortForwardingResponse{
		Id: pf.Id, Name: pf.Name, Direction: pf.Direction, Port: pf.Port,
		BindAddress: pf.BindAddress, TargetHost: pf.TargetHost, TargetPort: pf.TargetPort,
		SshConnectionId: pf.SshConnectionId, Status: running,
		LastError: lastErr, Remark: pf.Remark,
	}
}

// validate 校验规则字段；需关联已存在的 SSH 连接。
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
	if pf.SshConnectionId <= 0 {
		return fmt.Errorf("请选择 SSH 连接")
	}
	var conn SshConnection
	if result := config.GetDB().First(&conn, "id = ?", pf.SshConnectionId); result.Error != nil {
		return fmt.Errorf("SSH 连接不存在")
	}
	if strings.TrimSpace(pf.Name) == "" {
		if pf.Direction == DirectionRemote {
			pf.Name = fmt.Sprintf("[R] %s → %s", pf.listenAddr(), pf.targetAddr())
		} else {
			pf.Name = fmt.Sprintf("[L] %s → %s", pf.listenAddr(), pf.targetAddr())
		}
	}
	return nil
}

// ── Handlers ──────────────────────────────────────────────────────

func GetPortForwardings(c *gin.Context) {
	var rules []PortForwarding
	config.GetDB().Order("id ASC").Find(&rules)
	responses := make([]PortForwardingResponse, 0, len(rules))
	for i := range rules {
		// 存量行可能没有 direction，读路径同样按 local 兜底
		_ = rules[i].normalize()
		responses = append(responses, rules[i].toResponse())
	}
	c.JSON(200, responses)
}

func CreatePortForwarding(c *gin.Context) {
	var rule PortForwarding
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	rule.Status = false
	if err := rule.validate(); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if result := config.GetDB().Create(&rule); result.Error != nil {
		c.JSON(500, gin.H{"error": "创建失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "创建成功", "data": rule.toResponse()})
}

func UpdatePortForwarding(c *gin.Context) {
	id := c.Param("id")
	var rule PortForwarding
	if result := config.GetDB().First(&rule, "id = ?", id); result.Error != nil {
		c.JSON(404, gin.H{"error": "端口转发不存在"})
		return
	}
	if running, _ := GlobalPortForwarder.Status(rule.Id); running {
		c.JSON(400, gin.H{"error": "该转发正在运行，请先停止后再修改"})
		return
	}
	_ = rule.normalize()
	var req struct {
		Name            *string
		Direction       *string
		Port            *int
		BindAddress     *string
		TargetHost      *string
		TargetPort      *int
		SshConnectionId *int
		Remark          *string
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Direction != nil {
		rule.Direction = *req.Direction
	}
	if req.Port != nil {
		rule.Port = *req.Port
	}
	if req.BindAddress != nil {
		rule.BindAddress = *req.BindAddress
	}
	if req.TargetHost != nil {
		rule.TargetHost = *req.TargetHost
	}
	if req.TargetPort != nil {
		rule.TargetPort = *req.TargetPort
	}
	if req.SshConnectionId != nil {
		rule.SshConnectionId = *req.SshConnectionId
	}
	if req.Remark != nil {
		rule.Remark = *req.Remark
	}
	if err := rule.validate(); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if result := config.GetDB().Save(&rule); result.Error != nil {
		c.JSON(500, gin.H{"error": "更新失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "更新成功", "data": rule.toResponse()})
}

func DeletePortForwarding(c *gin.Context) {
	id := c.Param("id")
	var rule PortForwarding
	if result := config.GetDB().First(&rule, "id = ?", id); result.Error != nil {
		c.JSON(404, gin.H{"error": "端口转发不存在"})
		return
	}
	if running, _ := GlobalPortForwarder.Status(rule.Id); running {
		if err := GlobalPortForwarder.RemoveForward(rule.Id); err != nil {
			c.JSON(500, gin.H{"error": "停止端口转发失败: " + err.Error()})
			return
		}
	}
	if result := config.GetDB().Delete(&rule); result.Error != nil {
		c.JSON(500, gin.H{"error": "删除失败: " + result.Error.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "删除成功"})
}

func UpdatePortForwardingStatus(c *gin.Context) {
	id := c.Param("id")
	var rule PortForwarding
	if result := config.GetDB().First(&rule, "id = ?", id); result.Error != nil {
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
	running, _ := GlobalPortForwarder.Status(rule.Id)
	if req.Status {
		if running {
			c.JSON(400, gin.H{"error": "该端口转发已启动"})
			return
		}
		var conn SshConnection
		if result := config.GetDB().First(&conn, "id = ?", rule.SshConnectionId); result.Error != nil {
			c.JSON(400, gin.H{"error": "SSH 连接不存在，无法启动"})
			return
		}
		if err := GlobalPortForwarder.AddForward(&rule, &conn); err != nil {
			c.JSON(500, gin.H{"error": "启动端口转发失败: " + err.Error()})
			return
		}
		rule.Status = true
	} else {
		if !running {
			c.JSON(400, gin.H{"error": "该端口转发未启动"})
			return
		}
		if err := GlobalPortForwarder.RemoveForward(rule.Id); err != nil {
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
