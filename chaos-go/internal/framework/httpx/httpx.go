package httpx

import (
	"errors"
	"net/http"
	"strconv"

	renv "chaos-go/internal/framework/resp"
	"chaos-go/internal/framework/web"
)

func ParseID(c *web.Context) (int, bool) {
	return ParseParam(c, "id")
}

func ParseParam(c *web.Context, name string) (int, bool) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil {
		renv.Error(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}

type ErrRule struct {
	Err    error
	Status int
	Msg    string
}

func MapError(c *web.Context, err error, rules []ErrRule, fallback ...int) {
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
