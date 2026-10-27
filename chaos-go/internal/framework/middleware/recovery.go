package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

// Recovery 捕获处理链中的 panic 并转为统一错误信封（50000），
// 避免单个接口异常导致连接被直接断开、前端只看到网络错误。
//
// 典型场景：数据库尚未就绪时 config.GetDB() 返回 nil，依赖数据库的接口会在取值处空指针；
// 该中间件让调用方拿到明确的 50000，其它不依赖数据库的接口不受影响。
func Recovery() web.HandlerFunc {
	return func(c *web.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("接口 panic",
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"panic", fmt.Sprint(r),
					"stack", string(debug.Stack()))
				resp.ErrorCode(c, resp.CodeSystemError, "服务内部错误，请稍后重试")
				c.Abort()
			}
		}()
		c.Next()
	}
}
