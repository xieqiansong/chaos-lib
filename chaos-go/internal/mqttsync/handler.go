package mqttsync

import (
	"log/slog"
	"net/http"

	"chaos-go/internal/crud"
	renv "chaos-go/internal/resp"
	"chaos-go/internal/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/modules 导入该包而自动生效，无需 router.go 逐条编排。
func init() {
	routehub.Register("mqttSync", Register)
}

// Register 在路由组 rg 下挂载 MQTT 同步的全部接口：
//   - 标准 CRUD（list/get/create/update/delete）交给通用 crud，驱动前端 DataTable；
//     新建消息经 BeforeCreate 补齐标识、AfterCreate 广播到集群，即「发送」语义。
//   - 扩展能力（状态查询、按主题删除）由本包自实现并挂载到 /mqttSync 子路由。
func Register(rg *gin.RouterGroup) {
	crud.Register[MqttSyncMessage](rg, "mqttSync", crud.Opts[MqttSyncMessage]{
		Searchable:   []string{"channel", "node_id", "payload"},
		Sortable:     []string{"id", "created_at"},
		ToResponse:   toMessageDTOs,
		BeforeCreate: PrepareMessage,
		AfterCreate:  BroadcastMessage,
	})

	g := rg.Group("/mqttSync")
	g.GET("/status", status)
	g.DELETE("/channel", deleteChannel)
}

// status 处理 GET /api/mqttSync/status：暴露启用 / 连接状态与节点标识。
func status(c *gin.Context) {
	renv.Success(c, Status())
}

// deleteChannel 处理 DELETE /api/mqttSync/channel?name=xxx：
// 将该主题下的全部消息软删除（IsDeleted = true），仍保留在库中以便审计。
func deleteChannel(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		renv.Error(c, http.StatusBadRequest, "name is required")
		return
	}
	deleted, err := DeleteChannel(name)
	if err != nil {
		slog.Error("MQTT 主题消息删除失败", "channel", name, "err", err)
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, gin.H{"channel": name, "deleted": deleted})
}
