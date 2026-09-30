package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// openTempSQLite 打开一个临时 sqlite 文件，用作 GetDB 的替身。
func openTempSQLite(path string) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(path), &gorm.Config{})
}

// ── 配置解析类：纯函数，无副作用 ──

func TestDatabaseConfigGetDSN(t *testing.T) {
	cases := []struct {
		name string
		cfg  DatabaseConfig
		want string
	}{
		{
			name: "sqlite 直接返回文件路径",
			cfg:  DatabaseConfig{Type: "sqlite", Path: "data/chaos.db"},
			want: "data/chaos.db",
		},
		{
			name: "postgres 拼完整 DSN",
			cfg: DatabaseConfig{
				Type: "postgres", Host: "db.internal", Port: 5432,
				User: "chaos", Password: "secret", DBName: "chaos", SSLMode: "disable",
			},
			want: "host=db.internal user=chaos password=secret dbname=chaos port=5432 sslmode=disable",
		},
		{
			name: "非 sqlite 一律走 postgres 分支",
			cfg:  DatabaseConfig{Type: "mysql", Host: "h", User: "u", DBName: "d", Port: 3306, SSLMode: "require"},
			want: "host=h user=u password= dbname=d port=3306 sslmode=require",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cfg.GetDSN(); got != c.want {
				t.Fatalf("GetDSN() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAddressGetters(t *testing.T) {
	server := ServerConfig{Host: "0.0.0.0", Port: 8080}
	if got := server.GetAddress(); got != "0.0.0.0:8080" {
		t.Fatalf("ServerConfig.GetAddress() = %q", got)
	}
	pprof := PprofConfig{Host: "localhost", Port: 6060}
	if got := pprof.GetAddress(); got != "localhost:6060" {
		t.Fatalf("PprofConfig.GetAddress() = %q", got)
	}
}

func TestStringListUnmarshalText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want StringList
	}{
		{"去空白并丢弃空项", " a, b , ,c ", StringList{"a", "b", "c"}},
		{"单项", "t1", StringList{"t1"}},
		{"全空回到空列表", ",  ,", StringList{}},
		{"空串得到空列表", "", StringList{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got StringList
			if err := got.UnmarshalText([]byte(c.in)); err != nil {
				t.Fatalf("UnmarshalText 报错: %v", err)
			}
			// Len 为 0 时 got 可能是 nil 切片，统一按长度 + 元素比对
			if len(got) != len(c.want) || !reflect.DeepEqual(StringList(got), c.want) && len(c.want) > 0 {
				t.Fatalf("UnmarshalText(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}

func TestSupabaseConfigAvailable(t *testing.T) {
	full := SupabaseConfig{URL: "https://demo.supabase.co", SecretKey: "sb_secret_x", Tables: StringList{"t"}}
	if !full.Available() {
		t.Fatal("三项齐备时应可用")
	}
	for name, mutate := range map[string]func(c *SupabaseConfig){
		"缺 URL":    func(c *SupabaseConfig) { c.URL = "" },
		"缺 Secret": func(c *SupabaseConfig) { c.SecretKey = "" },
		"表名清单为空":   func(c *SupabaseConfig) { c.Tables = StringList{} },
	} {
		t.Run(name+"则不可用", func(t *testing.T) {
			c := full
			mutate(&c)
			if c.Available() {
				t.Fatalf("期望不可用，实际可用：%+v", c)
			}
		})
	}
}

// ── 数据库连接：全链路落在临时 sqlite 文件上，绝不碰开发/生产库 ──

// resetSingletons 把配置与连接两个单例复位，保证用例之间互不污染、且与执行顺序无关。
func resetSingletons(t *testing.T) {
	t.Helper()
	globalConfig = nil
	dbInstance = nil
	t.Cleanup(func() {
		globalConfig = nil
		ResetDBForTest()
	})
}

func TestGetDBConnectsSQLiteAndReusesInstance(t *testing.T) {
	resetSingletons(t)

	dbPath := filepath.Join(t.TempDir(), "nested", "chaos_test.db")
	globalConfig = &AppConfig{Database: DatabaseConfig{Type: "sqlite", Path: dbPath}}

	db := GetDB()
	if db == nil {
		t.Fatal("GetDB() 返回 nil")
	}
	// 注册在 TempDir 清理之后：LIFO 保证「先关连接、后删目录」，否则 Windows 上目录删不掉
	t.Cleanup(ResetDBForTest)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("sqlite 应已创建数据库文件: %v", err)
	}
	if GetDB() != db {
		t.Fatal("GetDB() 应复用同一个连接实例")
	}
	if err := db.Exec("CREATE TABLE ping (id integer primary key)").Error; err != nil {
		t.Fatalf("临时库上执行 SQL 失败: %v", err)
	}
}

func TestTryConnectDBReturnsErrorWhenPathUnusable(t *testing.T) {
	resetSingletons(t)

	// 把一个普通文件当目录用，MkdirAll 必然失败：稳定复现建连失败分支，且不会连到任何真实库
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("准备阻塞文件失败: %v", err)
	}
	globalConfig = &AppConfig{Database: DatabaseConfig{
		Type: "sqlite",
		Path: filepath.Join(blocker, "sub", "chaos_test.db"),
	}}

	db, err := TryConnectDB()
	if err == nil {
		t.Fatal("预期建连失败，实际成功了")
	}
	if db != nil {
		t.Fatal("失败时不应返回可用连接")
	}
	if dbInstance != nil {
		t.Fatal("失败时不应填充全局单例")
	}
}

func TestSetDBForTestIsolation(t *testing.T) {
	resetSingletons(t)

	sentinelDB, err := openTempSQLite(filepath.Join(t.TempDir(), "sentinel.db"))
	if err != nil {
		t.Fatalf("准备替身连接失败: %v", err)
	}
	// 同上：确保 TempDir 删除目录之前连接已关闭
	t.Cleanup(ResetDBForTest)
	SetDBForTest(sentinelDB)

	if GetDB() != sentinelDB {
		t.Fatal("注入后 GetDB() 应返回注入的连接")
	}
	if _, err := TryConnectDB(); err != nil {
		t.Fatalf("TryConnectDB 也应复用注入的连接: %v", err)
	}

	// 复位后单例应为空：此时若再取连接，走的必然是正常建连路径而非旧连接
	ResetDBForTest()
	if dbInstance != nil {
		t.Fatal("Reset 之后全局连接单例应为空")
	}
}
