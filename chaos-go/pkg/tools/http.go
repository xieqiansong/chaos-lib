package tools

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// defaultHTTPClient 带超时，避免请求永久挂起。
var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

// HTTPDo 发送一次 HTTP 请求，返回状态码与响应体（已自动关闭 resp.Body）。
// body 为 nil 表示无请求体；headers 为 nil 表示不设置额外请求头。
// 出错时返回的错误已包含网络/读写层面的问题，业务状态码需调用方自行判断。
func HTTPDo(method, url string, body []byte, headers map[string]string) (status int, respBody []byte, err error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, respBody, nil
}

// HTTPJSON 以 JSON 形式发送 payload（struct / map / slice 均可），
// 并把响应体解析进 out（传 nil 表示不解析）。默认设置 Content-Type: application/json。
// 返回状态码与错误，配合 tools.Must / tools.Must0 可在出错时直接中断：
//
//	status := tools.Must(tools.HTTPJSON("POST", url, payload, &result))
//	tools.Must0(tools.HTTPJSON("GET", url, nil, nil))
func HTTPJSON(method, url string, payload any, out any) (status int, err error) {
	var body []byte
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return 0, err
		}
	}
	status, respBody, err := HTTPDo(method, url, body, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return status, err
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return status, err
		}
	}
	return status, nil
}
