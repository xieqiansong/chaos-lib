// Package httpx 提供 handler 层的通用 HTTP 适配工具：路径参数解析与领域错误映射。
//
// 存量各业务包各自复制了两套骨架：
//   - `strconv.Atoi(c.Param("id"))` + 400 响应（全仓 29 处，taskplan 已私有抽出 parseID）；
//   - 「switch errors.Is(err, Xxx) → 状态码」的 writeError（4 份近似实现，仅错误表不同）。
//
// 这里收成一个解析函数与一张声明式规则表，业务包只保留自己的领域错误表。
package httpx

import (
	"errors"
	"net/http"
	"strconv"

	renv "chaos-go/internal/resp"

	"github.com/gin-gonic/gin"
)

// ParseID 解析路径参数 :id 为整数；失败时直接写 400 并返回 false，调用方据此 return。
func ParseID(c *gin.Context) (int, bool) {
	return ParseParam(c, "id")
}

// ParseParam 解析指定路径参数为整数；失败时直接写 400 并返回 false，调用方据此 return。
func ParseParam(c *gin.Context, name string) (int, bool) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}

// ErrRule 一条「领域错误 → HTTP 状态码」映射规则。Msg 为空时回落到 err.Error()。
type ErrRule struct {
	Err    error
	Status int
	Msg    string
}

// MapError 按规则表把领域错误映射为响应；未命中任何规则时以 fallback 状态（默认 500）返回。
func MapError(c *gin.Context, err error, rules []ErrRule, fallback ...int) {
	for _, r := range rules {
		if r.Err != nil && errors.Is(err, r.Err) {
			msg := r.Msg
			if msg == "" {
				msg = err.Error()
			}
			renv.Error(c, r.Status, msg)
			return
		}
	}
	status := http.StatusInternalServerError
	if len(fallback) > 0 {
		status = fallback[0]
	}
	renv.Error(c, status, err.Error())
}
