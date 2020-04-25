package rediscache

import (
	"dynamodb-golang-sample/internal/config"
	"dynamodb-golang-sample/internal/data"
	"dynamodb-golang-sample/internal/log"
	"encoding/json"
	"time"

	"github.com/gomodule/redigo/redis"
)

var pool *redis.Pool

var ttl int

// NewRedisCache is to set the configuration for redis
func NewRedisCache(cfg config.RedisConfig) {
	pool = newPool(cfg)
	ttl = cfg.TTL

	log.I("Successfully connected to redis cache: %v:%v (ttl: %v)", cfg.Host, cfg.Port, cfg.TTL)
}

// Close to disconnect the connection of redis
func Close() {
	pool.Close()
}

func newPool(cfg config.RedisConfig) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     cfg.PoolMaxIdle,
		MaxActive:   cfg.PoolMaxActive,
		IdleTimeout: time.Duration(cfg.PoolIdleTimeout) * time.Second,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			url := "redis://" + cfg.Host + ":" + cfg.Port
			return redis.DialURL(
				url,
				redis.DialPassword(cfg.Password),
				redis.DialConnectTimeout(time.Duration(cfg.ConnTimeout)*time.Millisecond),
			)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
}

// GetCache is to get the data from redis
func GetCache(key string) (*data.UserProfile, error) {
	c := pool.Get()
	defer c.Close()

	raw, err := redis.String(c.Do("GET", key))
	if err == redis.ErrNil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var value *data.UserProfile
	err = json.Unmarshal([]byte(raw), &value)
	if err != nil {
		log.E("%v: %v", key, err)
		return nil, err
	}
	return value, err
}

// SetCache is to record the data in redis
func SetCache(key string, value *data.UserProfile) (interface{}, error) {
	raw, err := json.Marshal(*value)
	if err != nil {
		log.E("%v: %v", key, err)
		return nil, err
	}

	c := pool.Get()
	defer c.Close()

	log.D("key: %s, value: %+v, ttl: %v", key, string(raw), ttl)

	if ttl == 0 {
		return c.Do("SET", key, raw)
	} else {
		return c.Do("SETEX", key, ttl, raw)
	}
}

// Del deletes key.
func Del(key string) error {
	c := pool.Get()
	defer c.Close()

	_, err := c.Do("DEL", key)
	return err
}
