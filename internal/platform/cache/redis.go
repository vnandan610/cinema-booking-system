package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrMiss = errors.New("cache miss")

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type Store interface {
	HealthChecker
	Close() error
	Delete(ctx context.Context, keys ...string) error
	GetJSON(ctx context.Context, key string, target any) error
	GetString(ctx context.Context, key string) (string, error)
	Keys(ctx context.Context, pattern string) ([]string, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	SetJSONPairIfAbsent(ctx context.Context, primaryKey string, primaryValue any, lookupKey string, lookupValue string, ttl time.Duration) (bool, error)
	SetString(ctx context.Context, key string, value string, ttl time.Duration) error
}

type RedisSettings struct {
	Addr         string
	Username     string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

type RedisStore struct {
	client *redis.Client
}

func OpenRedis(ctx context.Context, settings RedisSettings) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         settings.Addr,
		Username:     settings.Username,
		Password:     settings.Password,
		DB:           settings.DB,
		PoolSize:     settings.PoolSize,
		MinIdleConns: settings.MinIdleConns,
	})

	store := &RedisStore{client: client}
	if err := store.Ping(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return store, nil
}

func (store *RedisStore) Close() error {
	return store.client.Close()
}

func (store *RedisStore) Ping(ctx context.Context) error {
	return store.client.Ping(ctx).Err()
}

func (store *RedisStore) GetString(ctx context.Context, key string) (string, error) {
	value, err := store.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrMiss
	}
	return value, err
}

func (store *RedisStore) GetJSON(ctx context.Context, key string, target any) error {
	raw, err := store.GetString(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), target)
}

func (store *RedisStore) SetString(ctx context.Context, key string, value string, ttl time.Duration) error {
	return store.client.Set(ctx, key, value, ttl).Err()
}

func (store *RedisStore) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return store.client.Set(ctx, key, raw, ttl).Err()
}

func (store *RedisStore) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return store.client.Del(ctx, keys...).Err()
}

func (store *RedisStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := store.client.Scan(ctx, 0, pattern, 128).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys, iter.Err()
}

func (store *RedisStore) SetJSONPairIfAbsent(ctx context.Context, primaryKey string, primaryValue any, lookupKey string, lookupValue string, ttl time.Duration) (bool, error) {
	payload, err := json.Marshal(primaryValue)
	if err != nil {
		return false, err
	}

	result, err := setPairIfAbsent.Run(ctx, store.client, []string{primaryKey, lookupKey}, string(payload), lookupValue, ttl.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

var setPairIfAbsent = redis.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 1 then
	return 0
end
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[3])
redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
return 1
`)
