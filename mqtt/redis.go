package mqtt

import (
	"github.com/garyburd/redigo/redis"
	"log"
	"bytes"
    "encoding/gob"
	"sync"
	"fmt"
)

var g_redis_lock *sync.Mutex = new(sync.Mutex)

type RedisClient struct {
	Conn *redis.Conn
}

func StartRedisClient() *RedisClient {
	conn, err := redis.Dial("tcp", ":6379")

	if err != nil {
		log.Panicf("Failed to connect to Redis at port 6379")
	} else {
		log.Println("Redis client started")
	}

	client := new(RedisClient)
	client.Conn = &conn
	return client
}

func (client *RedisClient) Store(key string, value interface{}) {
	log.Printf("aqquiring g_redis_lock")
	g_redis_lock.Lock()
	defer g_redis_lock.Unlock()
	log.Printf("aqquired g_redis_lock")

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(value)
	if err != nil {
		log.Panic("gob encoding failed", err)
	}

	ret, err := (*client.Conn).Do("SET", key, buf.Bytes())
	if err != nil {
		log.Panic("redis failed to set key(%s): %s", key, err)
	}
	log.Printf("stored to redis, key=%s, val(some bytes), returned=%s",
		key, ret)
}

func (client *RedisClient) Fetch(key string, value interface{}) {
	log.Printf("aqquiring g_redis_lock")
	g_redis_lock.Lock()
	defer g_redis_lock.Unlock()
	log.Printf("aqquired g_redis_lock")

	str, err := redis.Bytes((*client.Conn).Do("GET", key))
	if err != nil {
		log.Printf("redis failed to fetch key(%s): %s",
			key, err)
		return
	}
	buf := bytes.NewBuffer(str)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(value)

	if (err != nil) {
		log.Panic("gob decode failed:", err)
	}
}

func (client *RedisClient) Delete(key string) {
	g_redis_lock.Lock()
	defer g_redis_lock.Unlock()
	(*client.Conn).Do("DEL", key)
}

func (client *RedisClient) GetFlyingMessagesForClient(client_id string) *map[uint16]FlyingMessage {
	key := fmt.Sprintf("gossipd.client-msg.%s", client_id)
	messages := make(map[uint16]FlyingMessage)
	client.Fetch(key, &messages)
	return &messages
}

func (client *RedisClient) SetFlyingMessagesForClient(client_id string,
	messages *map[uint16]FlyingMessage) {
	key := fmt.Sprintf("gossipd.client-msg.%s", client_id)
	client.Store(key, messages)
}

func (client *RedisClient) RemoveAllFlyingMessagesForClient(client_id string) {
	key := fmt.Sprintf("gossipd.client-msg.%s", client_id)
	g_redis_lock.Lock()
	defer g_redis_lock.Unlock()
	(*client.Conn).Do("DEL", key)
}

func (client *RedisClient) AddFlyingMessage(dest_id string,
	                                        fly_msg *FlyingMessage) {

	messages := *client.GetFlyingMessagesForClient(dest_id)
	messages[fly_msg.ClientMessageId] = *fly_msg
	client.SetFlyingMessagesForClient(dest_id, &messages)
	log.Printf("Added flying message to redis client:(%s), message_id:(%d)",
		dest_id, fly_msg.ClientMessageId)
}

func (client *RedisClient) Expire(key string, sec uint64) {
	g_redis_lock.Lock()
	defer g_redis_lock.Unlock()

	_, err := (*client.Conn).Do("EXPIRE", key, sec)
	if err != nil {
		log.Panic("failed to expire key(%s)", key)
	}
}