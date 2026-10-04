package crud

import (
	"fmt"
	"reflect"
	"strings"

	renv "chaos-go/internal/framework/resp"

	"github.com/gin-gonic/gin"
)

// firstElem 从 ToResponse 返回的切片中取出首元素（单条接口复用批量回调）。
// 是基线上唯一保留的反射点：ToResponse 的返回类型对包不可知，只能运行时取首元素。
func firstElem(v any) any {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.Len() > 0 {
		return rv.Index(0).Interface()
	}
	return v
}

// fail 统一错误响应：msg 可为 error 或任意值（字符串/拼接串），集中维护错误信封形态。
func fail(c *gin.Context, status int, msg any) {
	if e, ok := msg.(error); ok {
		renv.Error(c, status, e.Error())
		return
	}
	renv.Error(c, status, fmt.Sprintf("%v", msg))
}

// camelToSnake 把 Status 这类驼峰字段名转成 status，便于 Protected 同时覆盖两种写法。
func camelToSnake(s string) string {
	out := make([]rune, 0, len(s)+4)
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, '_')
		}
		out = append(out, r)
	}
	return strings.ToLower(string(out))
}
