package crud

import (
	"fmt"
	"reflect"
	"strings"

	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

func firstElem(v any) any {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.Len() > 0 {
		return rv.Index(0).Interface()
	}
	return v
}

func fail(c *web.Context, status int, msg any) {
	if e, ok := msg.(error); ok {
		renv.Error(c, status, e.Error())
		return
	}
	renv.Error(c, status, fmt.Sprintf("%v", msg))
}

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
