package routes

import (
	"testing"
	"testing/fstest"
)

// testWebFS 用内存 FS 代替前端构建产物，避免在测试里依赖真实打包输出。
var testWebFS = fstest.MapFS{
	"index.html":     &fstest.MapFile{Data: []byte("<html>chaos</html>")},
	"favicon.svg":    &fstest.MapFile{Data: []byte("<svg/>")},
	"assets/app.css": &fstest.MapFile{Data: []byte("body{}")},
}

// TestSetupRouterRegistersContract 固定 API 契约：
// 前端按约定路径调用，任一资源被改挂载点或删掉都应在这里失败。
// 这里只断言「路由已注册」，不发起请求——所有业务请求留给了各包自己的测试，
// 避免在本用例里触达数据库。
func TestSetupRouterRegistersContract(t *testing.T) {
	r := SetupRouter(testWebFS)

	want := []string{
		// 代理：浏览器数据 / 书签
		"GET /api/browserHistories",
		"POST /api/browserHistories",
		"POST /api/browserHistoryVisits",
		"GET /api/frequentBookmarks",
		"POST /api/bookmarks",
		// 代理：SDK 版本管理
		"GET /api/sdks",
		"GET /api/sdks/defs",
		"POST /api/sdks/defs",
		// 自包含资源包（标准 CRUD 基线）
		"GET /api/standardData",
		"GET /api/fileLinks",
		"GET /api/apiLog",
		"GET /api/dataCache",
		"GET /api/mqttSync",
		"GET /api/cronJob",
		"GET /api/projectGroups",
		"GET /api/projects",
		"GET /api/sshConns",
		"GET /api/portForwards",
		// 业务自定义动作
		"PATCH /api/projects/:id/move",
		"PATCH /api/projects/:id/access",
		"POST /api/cronJob/:id/run",
		"POST /api/cronJob/preview",
		"GET /api/dbMonitor/overview",
		"GET /api/dbMonitor/tables",
		// 快速编辑 / 环境变量
		"GET /api/quickEdits/",
		"GET /api/quickEdits/:id/content",
		"GET /api/envVariables/",
		"POST /api/envVariables/sync",
		// 任务计划与待办
		"GET /api/taskPlans/",
		"GET /api/taskPlans/tree",
		"GET /api/taskPlans/:id/tasks",
		"POST /api/ai/review-score",
		"GET /api/tasks/pending",
		"GET /api/tasks/contributionStats",
		// 内部周期任务的触发面
		"POST /api/systemJobs/sweep",
		"POST /api/systemJobs/portForwardSelfHeal",
		// 其它
		"POST /api/notify/",
		"GET /api/hostname",
		"GET /api/balance/deepseek",
		// 前端静态资源
		"GET /",
		"GET /favicon.svg",
		"GET /assets/*filepath",
	}

	registered := make(map[string]bool, len(want)*2)
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	for _, path := range want {
		if !registered[path] {
			t.Errorf("路由未注册：%s", path)
		}
	}
	for ri := range registered {
		t.Logf("已注册：%s", ri)
	}
}

// TestSetupRouterNoDuplicatePolicy 同一「方法 + 路径」重复注册通常是复制粘贴导致的覆盖。
func TestSetupRouterNoDuplicate(t *testing.T) {
	r := SetupRouter(testWebFS)

	seen := map[string]bool{}
	for _, ri := range r.Routes() {
		key := ri.Method + " " + ri.Path
		if seen[key] {
			t.Fatalf("重复注册路由：%s", key)
		}
		seen[key] = true
	}
}

func TestGetContentType(t *testing.T) {
	cases := map[string]string{
		"app.js":    "application/javascript; charset=utf-8",
		"app.css":   "text/css; charset=utf-8",
		"a.html":    "text/html; charset=utf-8",
		"a.json":    "application/json; charset=utf-8",
		"a.svg":     "image/svg+xml",
		"a.png":     "image/png",
		"a.jpg":     "image/jpeg",
		"a.woff2":   "font/woff2",
		"a.ttf":     "font/ttf",
		"unknown":   "application/octet-stream",
		"no_ext":    "application/octet-stream",
		"UPPER.CSS": "application/octet-stream", // 扩展名匹配区分大小写
	}
	for in, want := range cases {
		if got := getContentType(in); got != want {
			t.Fatalf("getContentType(%q) = %q, want %q", in, got, want)
		}
	}
}
