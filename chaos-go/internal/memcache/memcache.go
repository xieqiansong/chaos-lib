// Package memcache 是「内存缓存」：对齐 datacache（数据库缓存）的 (category, key) 复合键语义，
// 基于 hashicorp/golang-lru/v2 的 expirable LRU（容量上限 LRU 淘汰 + 逐条 TTL + 后台过期清理）。
//
// 与 datacache 相互独立：本包纯进程内存、进程重启即失效，适合高频读的短期数据；
// datacache 落库持久化，适合需跨进程/重启保留的数据。值统一为 []byte，与 datacache.Value 一致。
package memcache

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// memKey 复合键：(category, key)，与 datacache 的查询维度一致。
type memKey struct {
	Category string
	Key      string
}

// Cache 内存缓存实例。每个实例持有独立的容量上限与默认 TTL。
type Cache struct {
	lru *expirable.LRU[memKey, []byte]
}

// New 创建内存缓存实例：size 为容量上限（0 表示不限制，超过上限按 LRU 淘汰最久未用）；
// ttl 为默认过期时长（<=0 表示永不过期）。
func New(size int, ttl time.Duration) *Cache {
	return &Cache{lru: expirable.NewLRU[memKey, []byte](size, nil, ttl)}
}

// Set 写入缓存：覆盖同 (category, key) 的已有值，使用实例默认 TTL。
func (c *Cache) Set(category, key string, value []byte) {
	c.lru.Add(memKey{Category: category, Key: key}, value)
}

// Get 读取缓存：未命中或已过期返回 ok=false。
func (c *Cache) Get(category, key string) ([]byte, bool) {
	return c.lru.Get(memKey{Category: category, Key: key})
}

// Del 删除指定键，返回该键是否存在。
func (c *Cache) Del(category, key string) bool {
	return c.lru.Remove(memKey{Category: category, Key: key})
}

// Len 返回当前缓存条目数（含已过期尚未清理的条目）。
func (c *Cache) Len() int {
	return c.lru.Len()
}

// Default 默认实例：容量 1024、TTL 5 分钟。需要不同容量/TTL 的调用方用 New 自建实例。
var Default = New(1024, 5*time.Minute)

// Set 等价于 Default.Set，提供与 datacache.Set 一致的包级调用方式。
func Set(category, key string, value []byte) {
	Default.Set(category, key, value)
}

// Get 等价于 Default.Get。
func Get(category, key string) ([]byte, bool) {
	return Default.Get(category, key)
}

// Del 等价于 Default.Del。
func Del(category, key string) bool {
	return Default.Del(category, key)
}

// Len 等价于 Default.Len。
func Len() int {
	return Default.Len()
}
