package portfwd

import (
	"chaos-go/config"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// ── 模型 ──────────────────────────────────────────────────────────

// PortForwarding 端口转发规则：本地监听端口 → 经 SSH 隧道 → 远端目标 host:port。
type PortForwarding struct {
	Id              int    `gorm:"primaryKey"`
	Name            string
	Port            int
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
	Port            int
	TargetHost      string
	TargetPort      int
	SshConnectionId int
	Status          bool
	LastError       string
	Remark          string
}

// targetAddr 远端目标地址，由 SSH 服务器侧解析（等价 ssh -L 的目标段）。
func (pf *PortForwarding) targetAddr() string {
	return fmt.Sprintf("%s:%d", pf.TargetHost, pf.TargetPort)
}

func (pf *PortForwarding) toResponse() PortForwardingResponse {
	running, lastErr := GlobalPortForwarder.Status(pf.Port)
	return PortForwardingResponse{
		Id: pf.Id, Name: pf.Name, Port: pf.Port,
		TargetHost: pf.TargetHost, TargetPort: pf.TargetPort,
		SshConnectionId: pf.SshConnectionId, Status: running,
		LastError: lastErr, Remark: pf.Remark,
	}
}

// validate 校验规则字段；需关联已存在的 SSH 连接。
func (pf *PortForwarding) validate() error {
	if err := validatePort(pf.Port); err != nil {
		return fmt.Errorf("本地监听端口不合法: %v", err)
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
		pf.Name = fmt.Sprintf("%d → %s:%d", pf.Port, pf.TargetHost, pf.TargetPort)
	}
	return nil
}

// ── Handlers ──────────────────────────────────────────────────────

func GetPortForwardings(c *gin.Context) {
	var rules []PortForwarding
	config.GetDB().Order("id ASC").Find(&rules)
	responses := make([]PortForwardingResponse, 0, len(rules))
	for i := range rules {
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
	if running, _ := GlobalPortForwarder.Status(rule.Port); running {
		c.JSON(400, gin.H{"error": "该转发正在运行，请先停止后再修改"})
		return
	}
	var req struct {
		Name            *string
		Port            *int
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
	if req.Port != nil {
		rule.Port = *req.Port
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
	if running, _ := GlobalPortForwarder.Status(rule.Port); running {
		if err := GlobalPortForwarder.RemoveForward(rule.Port); err != nil {
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
	var req struct {
		Status bool
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	running, _ := GlobalPortForwarder.Status(rule.Port)
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
		if err := GlobalPortForwarder.RemoveForward(rule.Port); err != nil {
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
