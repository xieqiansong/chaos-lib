package portfwd

import (
	"chaos-go/config"
	"fmt"

	"github.com/gin-gonic/gin"
)

// ── 模型 ──────────────────────────────────────────────────────────

type PortForwarding struct {
	Id         int    `gorm:"primaryKey"`
	Name       string ``
	Port       int    ``
	TargetHost string ``
	TargetPort int    ``
	Status     bool   ``
}

func (PortForwarding) TableName() string {
	return "port_forwarding"
}

// ── Handlers ──────────────────────────────────────────────────────

func GetPortForwardings(context *gin.Context) {
	var forwardings []PortForwarding
	config.GetDB().Find(&forwardings)
	context.JSON(200, forwardings)
}

func SavePortForwarding(context *gin.Context) {
	var pf PortForwarding
	if err := context.ShouldBindJSON(&pf); err != nil {
		context.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if pf.Port <= 0 || pf.Port > 65535 {
		context.JSON(400, gin.H{"error": "端口号必须在1-65535之间"})
		return
	}
	if pf.TargetHost == "" || pf.TargetPort <= 0 || pf.TargetPort > 65535 {
		context.JSON(400, gin.H{"error": "目标主机和端口不能为空"})
		return
	}
	result := config.GetDB().Save(&pf)
	if result.Error != nil {
		context.JSON(500, gin.H{"error": "保存失败: " + result.Error.Error()})
		return
	}
	context.JSON(200, gin.H{"message": "保存成功", "data": pf})
}

func DeletePortForwarding(context *gin.Context) {
	id := context.Param("id")
	var pf PortForwarding
	result := config.GetDB().First(&pf, "id = ?", id)
	if result.Error != nil {
		context.JSON(404, gin.H{"error": "端口转发不存在"})
		return
	}
	if pf.Status {
		err := GlobalPortForwarder.RemoveForward(pf.Port)
		if err != nil {
			context.JSON(500, gin.H{"error": "停止端口转发失败: " + err.Error()})
			return
		}
	}
	result = config.GetDB().Delete(&pf)
	if result.Error != nil {
		context.JSON(500, gin.H{"error": "删除失败: " + result.Error.Error()})
		return
	}
	context.JSON(200, gin.H{"message": "删除成功"})
}

func UpdatePortForwardingStatus(context *gin.Context) {
	id := context.Param("id")
	var pf PortForwarding
	result := config.GetDB().First(&pf, "id = ?", id)
	if result.Error != nil {
		context.JSON(404, gin.H{"error": "端口转发不存在"})
		return
	}
	var request struct {
		Status bool
	}
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(400, gin.H{"error": err.Error()})
		return
	}
	targetAddr := fmt.Sprintf("%s:%d", pf.TargetHost, pf.TargetPort)
	if request.Status {
		if pf.Status {
			context.JSON(400, gin.H{"error": "端口转发已启动"})
			return
		}
		err := GlobalPortForwarder.AddForward(pf.Port, targetAddr)
		if err != nil {
			context.JSON(500, gin.H{"error": "启动端口转发失败: " + err.Error()})
			return
		}
		pf.Status = true
	} else {
		if !pf.Status {
			context.JSON(400, gin.H{"error": "端口转发已停止"})
			return
		}
		err := GlobalPortForwarder.RemoveForward(pf.Port)
		if err != nil {
			context.JSON(500, gin.H{"error": "停止端口转发失败: " + err.Error()})
			return
		}
		pf.Status = false
	}
	result = config.GetDB().Save(&pf)
	if result.Error != nil {
		context.JSON(500, gin.H{"error": "更新状态失败: " + result.Error.Error()})
		return
	}
	context.JSON(200, gin.H{"message": "状态更新成功", "data": pf})
}
