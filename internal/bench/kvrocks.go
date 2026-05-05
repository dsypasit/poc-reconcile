package bench

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Kvrocks struct {
	client *redis.Client
}

func NewKvrocks(addr string) *Kvrocks {
	return &Kvrocks{client: redis.NewClient(&redis.Options{Addr: addr})}
}

func (k *Kvrocks) Ping(ctx context.Context) error {
	if err := k.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping kvrocks: %w", err)
	}
	return nil
}

func (k *Kvrocks) Set(ctx context.Context, key, value string) error {
	return k.client.Set(ctx, key, value, 0).Err()
}

func (k *Kvrocks) Unlink(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return k.client.Unlink(ctx, keys...).Err()
}

func (k *Kvrocks) ScanPrefix(ctx context.Context, prefix string) ([]string, error) {
	pattern := prefix + "*"
	var cursor uint64
	keys := make([]string, 0, 1024)
	for {
		chunk, next, err := k.client.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, chunk...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

func (k *Kvrocks) Close() error {
	return k.client.Close()
}
