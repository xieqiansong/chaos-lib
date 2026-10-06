package mqttsync

import (
	"log/slog"
	"net/http"

	"chaos-go/internal/framework/crud"
	envelopepkg "chaos-go/internal/framework/envelope"
	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/routehub"

	"github.com/gin-gonic/gin"
)

// init 把本模块的路由挂载函数登记到 routehub，
// 使其随 internal/app 导入该包而自动生效，无需 router.go 逐条编排。
// 存量 /api 路由与新的 /api/v1 动作路由并存（双轨迁移）。
func init() {
	routehub.Register("mqtt-syncs", Register)
	routehub.RegisterV1("mqtt-syncs", RegisterV1)
}

// mqttSyncOpts 标准 CRUD 选项，存量 /api 与 v1 动作路由共用。
var mqttSyncOpts = crud.Opts[MqttSyncMessage]{
	Searchable:   []string{"channel", "node_id", "payload"},
	Sortable:     []string{"id", "created_at"},
	ToResponse:   toMessageDTOs,
	BeforeCreate: PrepareMessage,
	AfterCreate:  BroadcastMessage,
}

// Register 在路由组 rg 下挂载 MQTT 同步的全部接口：
//   - 标准 CRUD（list/get/create/update/delete）交给通用 crud，驱动前端 DataTable；
//     新建消息经 BeforeCreate 补齐标识、AfterCreate 广播到集群，即「发送」语义。
//   - 扩展能力（状态查询、按主题删除）由本包自实现并挂载到 /mqtt-syncs 子路由。
func Register(rg *gin.RouterGroup) {
	crud.Register[MqttSyncMessage](rg, "mqtt-syncs", mqttSyncOpts)

	g := rg.Group("/mqtt-syncs")
	g.GET("/status", status)
	g.DELETE("/channel", deleteChannel)
}

// RegisterV1 以「POST + Action」风格挂载到 /api/v1（详见《接口规范.md》）。
// CRUD 走通用 crud（list/get/create/update/delete/batchCreate/batchDelete）；
// 自定义动作：status（状态查询）、deleteChannel（按主题删除）亦迁移为 POST 动作。
func RegisterV1(rg *gin.RouterGroup) {
	g := crud.RegisterActions[MqttSyncMessage](rg, "mqtt-syncs", mqttSyncOpts, nil)
	g.POST("/status", status)
	g.POST("/deleteChannel", deleteChannelV1)
}

// status 处理状态查询（GET /api/mqtt-syncs/status 与 POST /api/v1/mqtt-syncs/status 共用）。
func status(c *gin.Context) {
	renv.Success(c, Status())
}

// deleteChannel 处理 DELETE /api/mqtt-syncs/channel?name=xxx：
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

// deleteChannelV1 是 deleteChannel 的「POST + Action」版（POST /api/v1/mqtt-syncs/deleteChannel，body {name}）。
func deleteChannelV1(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := envelopepkg.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name == "" {
		renv.Error(c, http.StatusBadRequest, "name is required")
		return
	}
	deleted, err := DeleteChannel(req.Name)
	if err != nil {
		slog.Error("MQTT 主题消息删除失败", "channel", req.Name, "err", err)
		renv.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	renv.Success(c, gin.H{"channel": req.Name, "deleted": deleted})
}
