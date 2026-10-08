// Package middleware 提供通用 gin 中间件（鉴权等），与路由解耦，按需挂载。
package middleware

import (
	"os"
	"strings"

	renv "chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

// AuthMiddleware 返回一个鉴权中间件：校验请求携带的访问令牌（token），
// 缺失 / 错误返回 40100（未登录），命中拒绝名单返回 40300（无权限）。
//
// 默认放行：未配置 CHAOS_API_TOKEN 时（本地开发 / 单用户场景）不校验，任何请求直接放行，
// 不影响存量接口。配置后，所有经本中间件的请求须携带 Authorization: Bearer <token>
// （或 X-Access-Token / 信封 meta.token），否则被拒，从而把「浏览器扩展 / 外部调用方」与
// 本地 UI 隔离在统一契约下。
//
// 令牌缺失 / 错误：40100；action 在 CHAOS_API_DENY_ACTIONS（逗号分隔，形如 "project.delete"）内：40300。
func AuthMiddleware() web.HandlerFunc {
	return func(c *web.Context) {
		expected := os.Getenv("CHAOS_API_TOKEN")
		if expected == "" {
			c.Next()
			return
		}
		token := extractToken(c)
		if token == "" || token != expected {
			resp.ErrorCode(c, resp.CodeUnauthorized, "未登录或凭证无效")
			c.Abort()
			return
		}
		// 越权检查：命中拒绝名单的 action 直接 403。
		if deny := os.Getenv("CHAOS_API_DENY_ACTIONS"); deny != "" {
			act := actionName(c)
			for _, d := range strings.Split(deny, ",") {
				if strings.TrimSpace(d) != "" && strings.TrimSpace(d) == act {
					resp.ErrorCode(c, resp.CodeForbidden, "无权限执行该操作")
					c.Abort()
					return
				}
			}
		}
		c.Next()
	}
}

// extractToken 依次从 Authorization: Bearer、X-Access-Token 头、信封 meta.token 提取令牌。
func extractToken(c *web.Context) string {
	if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	if t := c.GetHeader("X-Access-Token"); t != "" {
		return t
	}
	if m := renv.GetMetaMap(c); m != nil {
		if v, ok := m["token"].(string); ok {
			return v
		}
	}
	return ""
}

// actionName 取动作名：优先信封 action 字段（形如 module.act），退回 URL 末段（如 "delete"）。
func actionName(c *web.Context) string {
	if a := renv.GetAction(c); a != "" {
		return a
	}
	parts := strings.Split(strings.Trim(c.Request.URL.Path, "/"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return ""
}
