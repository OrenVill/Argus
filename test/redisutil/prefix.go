//go:build integration

package redisutil

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

// DeleteKeysWithPrefix removes all keys matching prefix* without flushing other DB data.
func DeleteKeysWithPrefix(ctx context.Context, rdb *goredis.Client, prefix string) error {
	var cursor uint64
	pattern := prefix + "*"
	for {
		keys, next, err := rdb.Scan(ctx, cursor, pattern, 256).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}
