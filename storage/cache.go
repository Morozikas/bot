// Пакет storage — хранение данных: Redis-кэш и PostgreSQL.
// cache.go — Redis-кэш с TTL для промежуточных данных (тикеры, OI, funding).
package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache — Redis-бэкенд для кэширования.
type Cache struct {
	client *redis.Client
}

// NewCache создаёт Cache из Redis URL.
func NewCache(redisURL string) (*Cache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Cache{client: client}, nil
}

// Set сохраняет JSON-сериализуемое значение с TTL.
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// Get десериализует значение из кэша в target.
func (c *Cache) Get(ctx context.Context, key string, target interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Del удаляет ключ.
func (c *Cache) Del(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Exists проверяет наличие ключа.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	return n > 0, err
}

// Close закрывает соединение с Redis.
func (c *Cache) Close() error { return c.client.Close() }
