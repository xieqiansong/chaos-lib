package mqttsync

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"chaos-go/config"
	"chaos-go/internal/crud"
	"github.com/gin-gonic/gin"
)

// Register 在路由组 rg 下挂载 MQTT 同步的全部接口：
//   - 标准 CRUD（list/get/create/update/delete）交给通用 crud，驱动前端 DataTable；
//     新建消息（create）经 AfterCreate 广播到集群，即「发送」语义。
//   - 扩展能力（状态查询、按主题删除）由本包自实现并挂载到 /mqttSync 子路由。
func Register(rg *gin.RouterGroup) {
	crud.Register(rg, "mqttSync", &MqttSyncMessage{}, crud.Opts{
		Searchable: []string{"channel", "node_id", "payload"},
		Sortable:   []string{"id", "created_at"},
		ToResponse: toMessageDTOs,
		// AfterCreate：新建消息即向集群广播（与落库共用同一 MsgID，便于对端去重）。
		// 通道未启用时返回错误，事务回滚，避免「本地已存却没广播」的误导。
		AfterCreate: func(row any) error {
			m, ok := row.(*MqttSyncMessage)
			if !ok {
				return nil
			}
			cfg := config.GetConfig().Mqtt
			if !cfg.Enabled {
				return errors.New("MQTT 未启用，无法发送")
			}
			wm := wireMessage{
				ID:      m.MsgID,
				NodeID:  m.NodeID,
				Channel: m.Channel,
				Payload: m.Payload,
				Ts:      time.Now().UTC().Format(time.RFC3339),
			}
			if err := Publish(wm); err != nil {
				// broker 离线：本地已落库，仅记日志，不回滚（与发布语义一致）
				slog.Warn("MQTT 消息广播失败（本地已落库）", "channel", m.Channel, "err", err)
			}
			return nil
		},
	})

	g := rg.Group("/mqttSync")
	g.GET("/status", Status)
	g.DELETE("/channel", DeleteChannel)
}

// Status 处理 GET /api/mqttSync/status：暴露启用 / 连接状态与节点标识。
func Status(c *gin.Context) {
	cfg := config.GetConfig().Mqtt
	c.JSON(http.StatusOK, gin.H{
		"enabled":   cfg.Enabled,
		"connected": IsConnected(),
		"broker":    cfg.Broker,
		"prefix":    cfg.Prefix,
		"node_id":   NodeID(),
		"encrypt":   EffectiveEncrypt(),
	})
}

// DeleteChannel 处理 DELETE /api/mqttSync/channel?name=xxx：
// 将该主题下的全部消息软删除（IsDeleted = true），仍保留在库中以便审计。
func DeleteChannel(c *gin.Context) {
	db := config.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": ErrDBUnavailable.Error()})
		return
	}
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	res := db.Model(&MqttSyncMessage{}).
		Where("channel = ? AND is_deleted = ?", name, false).
		Update("is_deleted", true)
	if res.Error != nil {
		slog.Error("MQTT 主题消息删除失败", "channel", name, "err", res.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"channel": name, "deleted": res.RowsAffected})
}
