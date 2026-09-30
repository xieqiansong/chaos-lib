package datacache

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"testing"
	"time"

	"chaos-go/internal/testkit"

	"gorm.io/gorm"
)

// newCacheDB 准备一个临时 sqlite 库并注入 config 单例，供 Set/Get 直接使用。
func newCacheDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testkit.NewTestDB(t, &DataCache{})
}

func TestSetAppliesDefaultsAndMetrics(t *testing.T) {
	db := newCacheDB(t)

	value := []byte("hello cache")
	if err := Set("demo", "k1", value, "", "", nil); err != nil {
		t.Fatalf("Set 报错: %v", err)
	}

	row, err := Get("demo", "k1")
	if err != nil {
		t.Fatalf("Get 报错: %v", err)
	}
	if row.DataType != "text" {
		t.Fatalf("data_type 缺省应回退 text，实际 %q", row.DataType)
	}
	if row.Compression != "none" {
		t.Fatalf("compression 缺省应回退 none，实际 %q", row.Compression)
	}
	if row.ValueLen != len(value) {
		t.Fatalf("value_len = %d, want %d", row.ValueLen, len(value))
	}
	wantMd5 := md5.Sum(value)
	if row.ValueMd5 != hex.EncodeToString(wantMd5[:]) {
		t.Fatalf("value_md5 = %q, want %q", row.ValueMd5, hex.EncodeToString(wantMd5[:]))
	}
	if string(row.Value) != "hello cache" {
		t.Fatalf("值往返失败：%q", row.Value)
	}
	if row.ExpireAt != nil {
		t.Fatal("未指定过期时间时应为空")
	}

	var n int64
	if err := db.Model(&DataCache{}).Count(&n).Error; err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("Set 应只写一条记录，实际 %d", n)
	}
}

func TestSetKeepsExplicitMeta(t *testing.T) {
	newCacheDB(t)

	expire := time.Now().Add(time.Hour).Truncate(time.Second)
	raw := []byte{0x00, 0x01, 0xff}
	if err := Set("demo", "binary", raw, "png", "zstd", &expire); err != nil {
		t.Fatalf("Set 报错: %v", err)
	}

	row, err := Get("demo", "binary")
	if err != nil {
		t.Fatalf("Get 报错: %v", err)
	}
	if row.DataType != "png" || row.Compression != "zstd" {
		t.Fatalf("显式指定的类型/算法被覆盖：%q / %q", row.DataType, row.Compression)
	}
	if !bytes.Equal(row.Value, raw) {
		t.Fatalf("二进制值往返失败：% x", row.Value)
	}
	if row.ValueLen != len(raw) {
		t.Fatalf("value_len = %d, want %d", row.ValueLen, len(raw))
	}
	if row.ExpireAt == nil {
		t.Fatal("过期时间未保留")
	}
	if !row.ExpireAt.Equal(expire) {
		t.Fatalf("过期时间 = %v, want %v", row.ExpireAt, expire)
	}
}

func TestGetReturnsLatestValue(t *testing.T) {
	db := newCacheDB(t)

	// Set 是「追加」语义：同键多次写入各留一条历史
	if err := Set("demo", "k", []byte("v1"), "", "", nil); err != nil {
		t.Fatalf("Set 报错: %v", err)
	}
	if err := Set("demo", "k", []byte("v2"), "", "", nil); err != nil {
		t.Fatalf("Set 报错: %v", err)
	}

	row, err := Get("demo", "k")
	if err != nil {
		t.Fatalf("Get 报错: %v", err)
	}
	if string(row.Value) != "v2" {
		t.Fatalf("Get 应返回最新一条，实际 %q", row.Value)
	}

	var n int64
	if err := db.Model(&DataCache{}).Count(&n).Error; err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if n != 2 {
		t.Fatalf("Set 不应覆盖历史记录，实际 %d 条", n)
	}
}

func TestGetNotFoundAndSkipsDeleted(t *testing.T) {
	db := newCacheDB(t)

	if _, err := Get("demo", "missing"); err == nil {
		t.Fatal("未命中应返回错误")
	}

	if err := Set("demo", "k", []byte("v"), "", "", nil); err != nil {
		t.Fatalf("Set 报错: %v", err)
	}
	// 直接标记软删，模拟走 Delete 通道回收的记录
	if err := db.Model(&DataCache{}).Where("category = ? AND key = ?", "demo", "k").
		Update("is_deleted", true).Error; err != nil {
		t.Fatalf("软删失败: %v", err)
	}

	if _, err := Get("demo", "k"); err == nil {
		t.Fatal("软删记录不应被 Get 命中")
	}
}

func TestGetIsScopedByCategoryAndKey(t *testing.T) {
	newCacheDB(t)

	if err := Set("alpha", "k", []byte("a"), "", "", nil); err != nil {
		t.Fatalf("Set 报错: %v", err)
	}
	if err := Set("beta", "k", []byte("b"), "", "", nil); err != nil {
		t.Fatalf("Set 报错: %v", err)
	}

	row, err := Get("beta", "k")
	if err != nil {
		t.Fatalf("Get 报错: %v", err)
	}
	if string(row.Value) != "b" {
		t.Fatalf("同名 key 不同 category 应互不影响，实际取到 %q", row.Value)
	}
}
