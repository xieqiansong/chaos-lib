package middleware

import (
	"os"
	"strings"

	renv "chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

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
