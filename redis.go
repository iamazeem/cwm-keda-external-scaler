package main

import (
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/go-redis/redis/v8"
)

var (
	rdb *redis.Client = nil
)

func connectToRedisServer() bool {
	// return true if already connected
	if rdb != nil {
		return true
	}

	log.Debug("connecting with Redis server")

	redisHost := getEnv(keyRedisHost, defaultRedisHost)
	redisPort := getEnv(keyRedisPort, defaultRedisPort)
	address := redisHost + ":" + redisPort

	// create new Redis client if one does not exist already
	redisDbStr := getEnv(keyRedisDb, defaultRedisDb)
	redisDb, err := strconv.Atoi(redisDbStr)
	if err != nil {
		redisDb = 0
		log.Warnf("invalid redis db %v. err: %v. using default db: %v", redisDbStr, err.Error(), redisDb)
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     address,
		Password: "", // no password set
		DB:       redisDb,
	})

	if !pingRedisServer() {
		rdb.Close()
		rdb = nil
		return false
	}

	log.Debugf("connected with Redis server [%v]", address)

	return true
}

func pingRedisServer() bool {
	log.Debug("pinging Redis server")

	val, err := rdb.Ping(rdb.Context()).Result()
	switch {
	case err == redis.Nil:
		return false
	case err != nil:
		log.Warnf("PING call failed! %v", err.Error())
		return false
	case val == "":
		log.Warn("empty value for 'PING'")
		return false
	case strings.ToUpper(val) != "PONG":
		log.Warn("PING != PONG")
		return false
	}

	log.Debugf("got: 'PING' = '%v'", val)

	return true
}

func getValueFromRedisServer(key string) (string, bool) {
	log.Debugf("getting '%v' from Redis server", key)

	if !connectToRedisServer() {
		log.Error("could not connect with Redis server")
		return "", false
	}

	val, err := rdb.Get(rdb.Context(), key).Result()
	switch {
	case err == redis.Nil:
		log.Errorf("key does not exist [%v]", key)
		return val, false
	case err != nil:
		log.Errorf("get call failed for '%v'! %v", key, err.Error())
		return val, false
	case val == "":
		log.Errorf("empty value for '%v'", key)
		return val, false
	}

	log.Debugf("got: [%v = %v]", key, val)

	return val, true
}
