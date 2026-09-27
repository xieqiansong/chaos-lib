package memcache

import (
	"bytes"
	"testing"
	"time"
)

// TestCacheSetGet 验证 Set/Get 基本读写与 (category, key) 维度隔离。
func TestCacheSetGet(t *testing.T) {
	c := New(16, time.Minute)
	c.Set("cat1", "k1", []byte("v1"))
	c.Set("cat2", "k1", []byte("v2")) // 同 key 不同 category，应互不影响

	v, ok := c.Get("cat1", "k1")
	if !ok || !bytes.Equal(v, []byte("v1")) {
		t.Fatalf("cat1/k1 期望 v1, got %q ok=%v", v, ok)
	}
	v, ok = c.Get("cat2", "k1")
	if !ok || !bytes.Equal(v, []byte("v2")) {
		t.Fatalf("cat2/k1 期望 v2, got %q ok=%v", v, ok)
	}
	if _, ok := c.Get("cat1", "missing"); ok {
		t.Fatal("未写入的键不应命中")
	}
}

// TestCacheOverwrite 验证同 (category, key) 重复 Set 覆盖旧值。
func TestCacheOverwrite(t *testing.T) {
	c := New(16, time.Minute)
	c.Set("c", "k", []byte("old"))
	c.Set("c", "k", []byte("new"))

	v, ok := c.Get("c", "k")
	if !ok || !bytes.Equal(v, []byte("new")) {
		t.Fatalf("期望被覆盖为 new, got %q ok=%v", v, ok)
	}
	if c.Len() != 1 {
		t.Fatalf("覆盖后条目数应为 1, got %d", c.Len())
	}
}

// TestCacheLRUEviction 验证容量超限后按 LRU 淘汰最久未用条目。
func TestCacheLRUEviction(t *testing.T) {
	c := New(2, time.Minute)
	c.Set("c", "k1", []byte("v1"))
	c.Set("c", "k2", []byte("v2"))
	c.Get("c", "k1") // 提升 k1 为最近使用
	c.Set("c", "k3", []byte("v3"))

	if _, ok := c.Get("c", "k2"); ok {
		t.Fatal("k2 应作为最久未用被淘汰")
	}
	if _, ok := c.Get("c", "k1"); !ok {
		t.Fatal("k1 被访问过，不应淘汰")
	}
	if _, ok := c.Get("c", "k3"); !ok {
		t.Fatal("k3 应存在")
	}
}

// TestCacheExpire 验证 TTL 到期后 Get 不再命中。
func TestCacheExpire(t *testing.T) {
	c := New(16, 50*time.Millisecond)
	c.Set("c", "k", []byte("v"))

	if _, ok := c.Get("c", "k"); !ok {
		t.Fatal("TTL 内应命中")
	}
	time.Sleep(120 * time.Millisecond)
	if _, ok := c.Get("c", "k"); ok {
		t.Fatal("过期后不应命中")
	}
}

// TestCacheDel 验证 Del 删除条目。
func TestCacheDel(t *testing.T) {
	c := New(16, time.Minute)
	c.Set("c", "k", []byte("v"))
	if !c.Del("c", "k") {
		t.Fatal("删除已存在键应返回 true")
	}
	if c.Del("c", "k") {
		t.Fatal("删除不存在键应返回 false")
	}
	if _, ok := c.Get("c", "k"); ok {
		t.Fatal("删除后不应命中")
	}
}
