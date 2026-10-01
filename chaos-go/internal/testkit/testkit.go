// Package testkit 为后端各包的测试提供统一的隔离基建。
//
// 两条硬性红线（本 package 的存在意义）：
//  1. 数据库：凡需要落库的测试，一律用 t.TempDir() 里的临时 sqlite 文件，
//     连接随测试注入 config 单例、随测试结束关闭，目录由 testing 框架自动删除。
//     开发/生产的 postgres / sqlite 库完全不被触碰。
//  2. 网络：不向外部发起任何请求；涉及 HTTP 的一律用 httptest 本地回环。
package testkit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"chaos-go/internal/config"
	renv "chaos-go/internal/resp"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestDB 建立一个临时 sqlite 库，按需迁移 models，并注入 config 全局单例。
// 返回的 *gorm.DB 就是之后 config.GetDB() 的返回值，业务代码无需任何改造。
func NewTestDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open temp sqlite failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap *sql.DB failed: %v", err)
	}
	// 与生产 sqlite 配置保持一致：单写连接，避免 database is locked
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign_keys failed: %v", err)
	}

	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			t.Fatalf("automigrate failed: %v", err)
		}
	}

	config.ResetDBForTest()
	config.SetDBForTest(db)
	// LIFO：先关闭连接，再让 t.TempDir() 删掉数据库文件（Windows 上必须如此）
	t.Cleanup(config.ResetDBForTest)
	return db
}

// Router 构造一个只带 /api 分组的 gin 引擎，路由由测试自己注册。
func Router(register func(api *gin.RouterGroup)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if register != nil {
		register(r.Group("/api"))
	}
	return r
}

// Do 发起一次本地 HTTP 请求（body 为空串时不带请求体）。不走 TCP，全部在内存里完成。
func Do(t *testing.T, r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// Envelope 统一响应信封（见 internal/resp）。
type Envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// Envelope 解析响应体为信封；非 JSON 直接失败。
func EnvelopeOf(t *testing.T, rec *httptest.ResponseRecorder) Envelope {
	t.Helper()

	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应不是合法 JSON（HTTP %d）: %v\nbody=%s", rec.Code, err, rec.Body.String())
	}
	return env
}

// RequireOK 断言业务码为 0（HTTP 200），返回信封里的 data 载荷。
func RequireOK(t *testing.T, rec *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP 期望 200，实际 %d；body=%s", rec.Code, rec.Body.String())
	}
	env := EnvelopeOf(t, rec)
	if env.Code != renv.CodeOK {
		t.Fatalf("code 期望 %d，实际 %d；message=%s", renv.CodeOK, env.Code, env.Message)
	}
	return env.Data
}

// RequireError 断言 HTTP 状态与业务码一致且等于 want，返回错误信息。
func RequireError(t *testing.T, rec *httptest.ResponseRecorder, want int) string {
	t.Helper()

	if rec.Code != want {
		t.Fatalf("HTTP 期望 %d，实际 %d；body=%s", want, rec.Code, rec.Body.String())
	}
	env := EnvelopeOf(t, rec)
	if env.Code != want {
		t.Fatalf("code 期望 %d，实际 %d；message=%s", want, env.Code, env.Message)
	}
	return env.Message
}
