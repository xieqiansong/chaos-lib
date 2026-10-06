package resp

// 业务错误码：替代 HTTP 状态码作为前后端错误判定的唯一依据。
// 规范见仓库根《接口规范.md》第 4 节。所有响应（含错误）HTTP 状态统一为 200，
// 由 code 区分错误类型，便于前端统一按 code 处理、网关按 code 路由，
// 彻底替代「用 HTTP 状态码表达错误」的旧写法。
const (
	CodeOK           = 0     // 成功
	CodeInvalidParam = 40001 // 参数错误
	CodeUnauthorized = 40100 // 未登录 / 凭证无效
	CodeForbidden    = 40300 // 已登录但无权限
	CodeNotFound     = 40400 // 数据 / 资源不存在
	CodeConflict     = 40900 // 冲突 / 版本不一致
	CodeSystemError  = 50000 // 系统错误
)

// statusToCode 把 HTTP 状态码映射为业务错误码；未知状态归为系统错误。
// 用于让存量 renv.Error(c, http.StatusXxx, msg) 调用零改动地获得业务码。
func statusToCode(status int) int {
	switch status {
	case 400, 422:
		return CodeInvalidParam
	case 401:
		return CodeUnauthorized
	case 403:
		return CodeForbidden
	case 404:
		return CodeNotFound
	case 409:
		return CodeConflict
	case 500, 502, 503, 504:
		return CodeSystemError
	default:
		if status >= 500 {
			return CodeSystemError
		}
		return CodeInvalidParam
	}
}
