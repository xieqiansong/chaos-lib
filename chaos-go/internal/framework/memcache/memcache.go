package memcache

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type memKey struct {
	Category string
	Key      string
}

type Cache struct {
	lru *expirable.LRU[memKey, []byte]
}

func New(size int, ttl time.Duration) *Cache {
	return &Cache{lru: expirable.NewLRU[memKey, []byte](size, nil, ttl)}
}

func (c *Cache) Set(category, key string, value []byte) {
	c.lru.Add(memKey{Category: category, Key: key}, value)
}

func (c *Cache) Get(category, key string) ([]byte, bool) {
	return c.lru.Get(memKey{Category: category, Key: key})
}

func (c *Cache) Del(category, key string) bool {
	return c.lru.Remove(memKey{Category: category, Key: key})
}

func (c *Cache) Len() int {
	return c.lru.Len()
}

var Default = New(1024, 5*time.Minute)

func Set(category, key string, value []byte) {
	Default.Set(category, key, value)
}

func Get(category, key string) ([]byte, bool) {
	return Default.Get(category, key)
}

func Del(category, key string) bool {
	return Default.Del(category, key)
}

func Len() int {
	return Default.Len()
}
