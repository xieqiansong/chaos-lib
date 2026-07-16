package services

import (
	"chaos-go/tools"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ShowNotify(context *gin.Context) {
	tools.SendMessage("通知", "您有一条新通知。")

	context.JSON(http.StatusOK, gin.H{
		"result":  "sent",
		"message": "已通过 WxPusher 发送通知",
	})
}
