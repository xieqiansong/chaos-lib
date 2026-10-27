package resp

import (
	"net/http"

	renv "chaos-go/internal/framework/envelope"
	"chaos-go/internal/framework/web"
)

func Success(c *web.Context, data interface{}) {
	c.JSON(http.StatusOK, envelope(c, CodeOK, "ok", data))
}

func SuccessMsg(c *web.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, envelope(c, CodeOK, message, data))
}

func Error(c *web.Context, status int, message string) {
	if status < 400 {
		status = 400
	}
	c.JSON(http.StatusOK, envelope(c, statusToCode(status), message, nil))
}

func ErrorCode(c *web.Context, code int, message string) {
	if code < 0 {
		code = CodeSystemError
	}
	c.JSON(http.StatusOK, envelope(c, code, message, nil))
}

func envelope(c *web.Context, code int, message string, data interface{}) renv.Response {
	return renv.Response{
		Code:      code,
		Message:   message,
		Data:      data,
		RequestID: renv.RequestID(c),
		Timestamp: renv.Timestamp(),
	}
}
