package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb *redis.Client
}

func New(addr string) *Cache {
	return &Cache{rdb: redis.NewClient(&redis.Options{Addr: addr})}
}

func (c *Cache) SetNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, val, ttl).Result()
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

func (c *Cache) Close() error {
	return c.rdb.Close()
}
