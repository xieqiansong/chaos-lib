package tools

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

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
