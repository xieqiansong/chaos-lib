// Package resp 提供统一的 HTTP 响应信封，全站所有接口（普通 / 分页 / 错误）
// 都通过本包返回，确保前后端契约一致。
//
// 统一信封：
//
//	{ "code": 0, "message": "ok", "data": <任意载荷>, "requestId": "<uuid>", "timestamp": 1710000000000 }
//
// 约定：
//   - 成功 code 恒为 0，message 成功时为 "ok"；
//   - 失败 code 取 HTTP 状态码（400/404/500…，后续阶段将切换为规范业务错误码
//     40001/40100/40300/40400/40900/50000），message 为可读错误信息；
//   - requestId / timestamp 由框架自动填充，便于链路追踪与幂等；存量接口未带信封时
//     也会获得服务端生成的 requestId（纯增量，前端只按 code/message/data 解包，不受影响）；
//   - 分页接口的 data 为 { list, pagination }，见 internal/pagination。
//
// 前端 src/utils/api.ts 的 sendMessage 会按此信封解包：code != 0 抛错，
// 否则返回 data；待迁移完成后将同时透传 requestId / timestamp。
package resp

import (
	renv "chaos-go/internal/framework/envelope"

	"github.com/gin-gonic/gin"
)

// CodeOK 成功码，全站统一为 0。
const CodeOK = 0

// Success 返回成功信封，data 可为任意载荷（分页时为 pagination.PageData）。
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, envelope(c, CodeOK, "ok", data))
}

// SuccessMsg 与 Success 类似，但允许自定义成功 message（极少数场景使用）。
func SuccessMsg(c *gin.Context, message string, data interface{}) {
	c.JSON(200, envelope(c, CodeOK, message, data))
}

// Error 返回错误信封，HTTP 状态与 code 保持一致（便于前端按 code 判错）。
func Error(c *gin.Context, status int, message string) {
	if status < 400 {
		status = 400
	}
	c.JSON(status, envelope(c, status, message, nil))
}

// envelope 组装统一响应信封，自动填充 requestId 与 timestamp。
func envelope(c *gin.Context, code int, message string, data interface{}) renv.Response {
	return renv.Response{
		Code:      code,
		Message:   message,
		Data:      data,
		RequestID: renv.RequestID(c),
		Timestamp: renv.Timestamp(),
	}
}
