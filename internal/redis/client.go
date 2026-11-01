package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/OrenVill/Argus/internal/config"
)

// Client wraps go-redis with Argus connection defaults.
type Client struct {
	RDB *goredis.Client
}

// NewClient dials Redis from config.
func NewClient(cfg config.Redis) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr: cfg.Address,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis ping %s: %w", cfg.Address, err)
	}
	return &Client{RDB: rdb}, nil
}

// Close releases the connection pool.
func (c *Client) Close() error {
	if c == nil || c.RDB == nil {
		return nil
	}
	return c.RDB.Close()
}
