package main

import (
	"log"
	"strings"

	"github.com/go-redis/redis/v8"
)

var (
	rdb *redis.Client = nil
)

func connectToRedisServer() bool {
	log.Printf("establishing connection with Redis server")

	redisHost := getEnv(keyRedisHost, defaultRedisHost)
	redisPort := getEnv(keyRedisPort, defaultRedisPort)
	address := redisHost + ":" + redisPort

	// check if the existing Redis server's address <host:port> changed
	// close the existing connection and cleanup
	// and try to connect with the new Redis server
	if rdb != nil && address != rdb.Options().Addr {
		log.Printf("address of Redis server changed from %v to %v", rdb.Options().Addr, address)
		log.Printf("previous Redis connection will be closed and the new one will be established")

		rdb.Close()
		rdb = nil
	}

	// create new Redis client if one does not exist already
	if rdb == nil {
		rdb = redis.NewClient(&redis.Options{
			Addr:     address,
			Password: "", // no password set
			DB:       0,  // use default DB
		})
	}

	if !pingRedisServer() {
		rdb.Close()
		rdb = nil
		return false
	}

	log.Printf("successful connection with Redis server [%v]", address)

	return true
}

func pingRedisServer() bool {
	log.Println("pinging Redis server")

	val, err := rdb.Ping(rdb.Context()).Result()
	switch {
	case err == redis.Nil:
		return false
	case err != nil:
		log.Printf("PING call failed! %v", err.Error())
		return false
	case val == "":
		log.Println("empty value for 'PING'")
		return false
	case strings.ToUpper(val) != "PONG":
		log.Println("PING != PONG")
		return false
	}

	log.Printf("Redis server replied: '%v'", val)

	return true
}

func getValueFromRedisServer(key string) (string, bool) {
	log.Printf("getting value for '%v' key from Redis server", key)

	if !connectToRedisServer() {
		log.Println("could not connect with Redis server")
		return "", false
	}

	val, err := rdb.Get(rdb.Context(), key).Result()
	switch {
	case err == redis.Nil:
		log.Printf("'%v' key does not exist", key)
		return val, false
	case err != nil:
		log.Printf("get call failed for '%v'! %v", key, err.Error())
		return val, false
	case val == "":
		log.Printf("empty value for '%v'", key)
		return val, false
	}

	log.Printf("Redis server returned: '%v'", val)

	return val, true
}
