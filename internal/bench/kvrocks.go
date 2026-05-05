package bench

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Kvrocks struct {
	primaryAddr string
	mu          sync.Mutex
	clients     map[string]*redis.Client
}

func NewKvrocks(addr string) *Kvrocks {
	return &Kvrocks{
		primaryAddr: addr,
		clients: map[string]*redis.Client{
			addr: redis.NewClient(&redis.Options{Addr: addr}),
		},
	}
}

func (k *Kvrocks) Ping(ctx context.Context) error {
	if err := k.clientForAddr(k.primaryAddr).Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping kvrocks: %w", err)
	}
	return nil
}

func (k *Kvrocks) Set(ctx context.Context, key, value string) error {
	return k.withRedirectRetry(ctx, k.primaryAddr, func(c *redis.Client) error {
		return c.Set(ctx, key, value, 0).Err()
	})
}

func (k *Kvrocks) SetEX(ctx context.Context, key, value string, ttl time.Duration) error {
	return k.withRedirectRetry(ctx, k.primaryAddr, func(c *redis.Client) error {
		return c.Set(ctx, key, value, ttl).Err()
	})
}

func (k *Kvrocks) Get(ctx context.Context, key string) (string, error) {
	var out string
	err := k.withRedirectRetry(ctx, k.primaryAddr, func(c *redis.Client) error {
		v, err := c.Get(ctx, key).Result()
		if err == redis.Nil {
			out = ""
			return nil
		}
		if err != nil {
			return err
		}
		out = v
		return nil
	})
	if err != nil {
		return "", err
	}
	return out, nil
}

func (k *Kvrocks) Unlink(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	var firstErr error
	for _, key := range keys {
		err := k.withRedirectRetry(ctx, k.primaryAddr, func(c *redis.Client) error {
			return c.Unlink(ctx, key).Err()
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (k *Kvrocks) ScanPrefix(ctx context.Context, prefix string) ([]string, error) {
	pattern := prefix + "*"
	seen := make(map[string]struct{}, 1024)
	for _, client := range k.allClients() {
		var cursor uint64
		for {
			chunk, next, err := client.Scan(ctx, cursor, pattern, 500).Result()
			if err != nil {
				return nil, err
			}
			for _, key := range chunk {
				seen[key] = struct{}{}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	return keys, nil
}

func (k *Kvrocks) Close() error {
	var firstErr error
	for _, client := range k.allClients() {
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (k *Kvrocks) withRedirectRetry(ctx context.Context, addr string, fn func(*redis.Client) error) error {
	client := k.clientForAddr(addr)
	err := fn(client)
	redirectAddr, moved := movedTarget(err)
	if !moved {
		return err
	}
	redirectAddr = normalizeRedirectAddr(redirectAddr, k.primaryAddr)
	return fn(k.clientForAddr(redirectAddr))
}

func movedTarget(err error) (string, bool) {
	if err == redis.Nil {
		return "", false
	}
	if err == nil {
		return "", false
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "MOVED ") {
		return "", false
	}
	parts := strings.Fields(msg)
	if len(parts) != 3 {
		return "", false
	}
	return parts[2], true
}

func (k *Kvrocks) clientForAddr(addr string) *redis.Client {
	k.mu.Lock()
	defer k.mu.Unlock()
	if client, ok := k.clients[addr]; ok {
		return client
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	k.clients[addr] = client
	return client
}

func (k *Kvrocks) allClients() []*redis.Client {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := make([]*redis.Client, 0, len(k.clients))
	for _, client := range k.clients {
		out = append(out, client)
	}
	return out
}

func normalizeRedirectAddr(redirectAddr, primaryAddr string) string {
	rHost, rPort, err := net.SplitHostPort(redirectAddr)
	if err != nil {
		return redirectAddr
	}
	pHost, _, err := net.SplitHostPort(primaryAddr)
	if err != nil {
		return redirectAddr
	}
	if net.ParseIP(rHost) != nil || rHost == "localhost" || rHost == pHost {
		return redirectAddr
	}
	return net.JoinHostPort(pHost, rPort)
}
