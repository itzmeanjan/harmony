package graph

import (
	"context"
	"errors"
	"log"
	"regexp"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/config"
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

// SubscribeToPendingTxEntry - Subscribe to topic where new pending tx(s)
// are published
func SubscribeToPendingTxEntry(ctx context.Context) (*redis.PubSub, error) {

	return SubscribeToTopic(ctx, config.GetPendingTxEntryPublishTopic())

}

// SubscribeToQueuedTxEntry - Subscribe to topic where new queued tx(s)
// are published
func SubscribeToQueuedTxEntry(ctx context.Context) (*redis.PubSub, error) {

	return SubscribeToTopic(ctx, config.GetQueuedTxEntryPublishTopic())

}

// ListenToMessages - Attempts to listen to messages being published
// on topic to which graphQL client has subscribed to over websocket transport
//
// This can be run as a seperate go routine
func ListenToMessages(ctx context.Context, pubsub *redis.PubSub, comm chan<- *model.MemPoolTx) {

	{
	OUTER:
		for {

			msg, err := pubsub.ReceiveTimeout(ctx, time.Second)
			if err != nil {
				continue
			}

			switch m := msg.(type) {

			case *redis.Subscription:

				// Pubsub broker informed we've been unsubscribed from
				// this topic
				//
				// It's better to leave this infinite loop
				if m.Kind == "unsubscribe" {
					break OUTER
				}

			case *redis.Message:

				// New message has been published on topic
				// of our interest, we'll attempt to deserialize
				// data to deliver it to client in expected format
				message := UnmarshalPubSubMessage([]byte(m.Payload))
				if message != nil {
					comm <- message.ToGraphQL()
				}

			}

		}
	}

}

// UnmarshalPubSubMessage - Attempts to unmarshal message pack serialized
// pubsub message as structured tx data, which is to be sent to subscriber
func UnmarshalPubSubMessage(message []byte) *data.MemPoolTx {

	_message, err := data.FromMessagePack(message)
	if err != nil {
		return nil
	}

	return _message

}
