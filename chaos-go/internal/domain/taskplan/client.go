package taskplan

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const rawFetchTimeout = 15 * time.Second

func fetchRawContent(rawURL string) (string, error) {
	client := &http.Client{Timeout: rawFetchTimeout}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("状态码 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
