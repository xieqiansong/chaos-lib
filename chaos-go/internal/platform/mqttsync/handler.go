package mqttsync

import (
	"log/slog"
	"net/http"

	"chaos-go/internal/framework/crud"
	envelopepkg "chaos-go/internal/framework/envelope"
	renv "chaos-go/internal/framework/resp"

	"chaos-go/internal/framework/web"
)

var mqttSyncOpts = crud.Opts[MqttSyncMessage]{
	Searchable: []string{"channel", "node_id", "payload"},
	Sortable:   []string{"id", "updated_at"},
	ToResponse: toMessageDTOs,
	V1CreateHandler: createAndBroadcast,
}

func RegisterV1(rg *web.RouterGroup) {
	g := crud.RegisterActions[MqttSyncMessage](rg, "mqtt-syncs", mqttSyncOpts, nil)
	g.POST("/status", status)
	g.POST("/deleteChannel", deleteChannelV1)
}

// createAndBroadcast 自定义 create：Prepare → Upsert（落库去重）→ Broadcast（MQTT）
func createAndBroadcast(c *web.Context) {
	var req MqttSyncMessage
	if err := envelopepkg.Bind(c, &req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := PrepareMessage(&req); err != nil {
		renv.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	rec, isNew, err := UpsertMessage(req.NodeID, req.Channel, req.Payload, req.MsgID)
	if err != nil {
		slog.Error("MQTT 消息 upsert 落库失败", "err", err)
		renv.Error(c, http.StatusInternalServerError, "落库失败: "+err.Error())
		return
	}

	// 无论新建还是更新，都广播到 MQTT
	if err := BroadcastMessage(&rec); err != nil {
		// BroadcastMessage 内部已处理 MQTT 不可用的情况
		slog.Warn("MQTT 广播失败（本地已落库）", "channel", rec.Channel, "isNew", isNew, "err", err)
	}

	renv.Success(c, toDTO(rec))
}

func status(c *web.Context) {
	renv.Success(c, Status())
}

func deleteChannel(c *web.Context) {
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
	renv.Success(c, map[string]any{"channel": name, "deleted": deleted})
}

func deleteChannelV1(c *web.Context) {
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
	renv.Success(c, map[string]any{"channel": req.Name, "deleted": deleted})
}
