// Package resp 提供统一的 HTTP 响应信封，全站所有接口（普通 / 分页 / 错误）
// 都通过本包返回，确保前后端契约一致。
//
// 统一信封：
//
//	{ "code": 0, "message": "ok", "data": <任意载荷> }
//
// 约定：
//   - 成功 code 恒为 0，message 成功时为 "ok"；
//   - 失败 code 取 HTTP 状态码（400/404/500…），message 为可读错误信息；
//   - 分页接口的 data 为 { list, pagination }，见 internal/pagination。
//
// 前端 src/utils/api.ts 的 sendMessage 会按此信封解包：code != 0 抛错，
// 否则返回 data；对尚未来得及迁移的旧接口做「无 code 字段则原样返回」的兼容。
package resp

import "github.com/gin-gonic/gin"

// CodeOK 成功码，全站统一为 0。
const CodeOK = 0

// Success 返回成功信封，data 可为任意载荷（分页时为 pagination.PageData）。
func Success(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{"code": CodeOK, "message": "ok", "data": data})
}

// SuccessMsg 与 Success 类似，但允许自定义成功 message（极少数场景使用）。
func SuccessMsg(c *gin.Context, message string, data interface{}) {
	c.JSON(200, gin.H{"code": CodeOK, "message": message, "data": data})
}

// Error 返回错误信封，HTTP 状态与 code 保持一致（便于前端按 code 判错）。
func Error(c *gin.Context, status int, message string) {
	if status < 400 {
		status = 400
	}
	c.JSON(status, gin.H{"code": status, "message": message})
}
