package pkg

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func OpenRedis(url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	cli := redis.NewClient(opt)
	if err := cli.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return cli, nil
}

const RemindZSetKey = "todo:remind"