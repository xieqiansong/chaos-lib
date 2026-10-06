// Package resp 提供统一的 HTTP 响应信封，全站所有接口（普通 / 分页 / 错误）
// 都通过本包返回，确保前后端契约一致。
//
// 统一信封：
//
//	{ "code": 0, "message": "ok", "data": <任意载荷>, "requestId": "<uuid>", "timestamp": 1710000000000 }
//
// 约定：
//   - 成功 code 恒为 0，message 成功时为 "ok"；
//   - 失败 code 取规范业务错误码（见 code.go：40001 参数错误 / 40100 未登录 /
//     40300 无权限 / 40400 不存在 / 40900 冲突 / 50000 系统错误），message 为可读错误信息；
//   - 所有响应（含错误）HTTP 状态统一为 200，错误完全由 code 区分，便于前端按 code 统一处理、
//     网关按 code 路由，彻底替代「用 HTTP 状态码表达错误」的写法；
//   - requestId / timestamp 由框架自动填充，便于链路追踪与幂等；存量接口未带信封时
//     也会获得服务端生成的 requestId（纯增量，前端只按 code/message/data 解包，不受影响）；
//   - 分页接口的 data 为 { list, pagination }，见 internal/pagination。
package resp

import (
	"net/http"

	renv "chaos-go/internal/framework/envelope"

	"github.com/gin-gonic/gin"
)

// Success 返回成功信封，data 可为任意载荷（分页时为 pagination.PageData）。
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, envelope(c, CodeOK, "ok", data))
}

// SuccessMsg 与 Success 类似，但允许自定义成功 message（极少数场景使用）。
func SuccessMsg(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, envelope(c, CodeOK, message, data))
}

// Error 返回错误信封。HTTP 状态统一为 200，code 由 status 经 statusToCode 映射为
// 规范业务错误码（见 code.go）。存量调用 renv.Error(c, http.StatusXxx, msg) 无需改动即可
// 获得业务码；新增代码建议直接用 ErrorCode 显式传业务码。
func Error(c *gin.Context, status int, message string) {
	if status < 400 {
		status = 400
	}
	c.JSON(http.StatusOK, envelope(c, statusToCode(status), message, nil))
}

// ErrorCode 返回错误信封并直接指定业务错误码（如 resp.CodeUnauthorized = 40100），
// HTTP 状态统一为 200。用于需要精确业务码的场景（如鉴权中间件）。
func ErrorCode(c *gin.Context, code int, message string) {
	if code < 0 {
		code = CodeSystemError
	}
	c.JSON(http.StatusOK, envelope(c, code, message, nil))
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
