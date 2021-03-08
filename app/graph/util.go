package graph

import (
	"context"
	"errors"
	"log"
	"regexp"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/data"
	"github.com/itzmeanjan/harmony/app/graph/model"
)

var memPool *data.MemPool
var redisClient *redis.Client

// InitMemPool - Initializing mempool handle, in this module
// so that it can be used before responding back to graphql queries
func InitMemPool(pool *data.MemPool) error {

	if pool != nil {
		memPool = pool
		return nil
	}

	return errors.New("Bad mempool received in graphQL handler")

}

// InitRedisClient - Initializing redis client handle, so that all
// subscriptions can be done using this client
func InitRedisClient(client *redis.Client) error {

	if client != nil {
		redisClient = client
		return nil
	}

	return errors.New("Bad redis client received in graphQL handler")

}

// Given a list of mempool tx(s), convert them to
// compatible list of graphql tx(s)
func toGraphQL(txs []*data.MemPoolTx) []*model.MemPoolTx {

	res := make([]*model.MemPoolTx, 0, len(txs))

	for _, tx := range txs {

		res = append(res, tx.ToGraphQL())

	}

	return res

}

// Attempts to parse duration, obtained from user query
func parseDuration(d string) (time.Duration, error) {

	dur, err := time.ParseDuration(d)
	if err != nil {
		return time.Duration(0), err
	}

	return dur, nil

}

// Checks whether received string is valid address or not
func checkAddress(address string) bool {

	reg, err := regexp.Compile(`^(0x\w{40})$`)
	if err != nil {

		log.Printf("[!] Failed to compile regular expression : %s\n", err.Error())
		return false

	}

	return reg.MatchString(address)

}

// SubscribeToTopic - Subscribes to Redis topic, with context of caller
// while waiting for subscription confirmation
func SubscribeToTopic(ctx context.Context, topic string) (*redis.PubSub, error) {

	_pubsub := redisClient.Subscribe(ctx, topic)

	// Waiting for subscription confirmation
	_, err := _pubsub.Receive(ctx)
	if err != nil {
		return nil, err
	}

	return _pubsub, nil

}
