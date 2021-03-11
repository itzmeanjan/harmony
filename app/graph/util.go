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
var parentCtx context.Context

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

// InitParentContext - Initializing parent context, to be listened by all
// graphQL subscribers so that graceful shutdown can be done
func InitParentContext(ctx context.Context) {
	parentCtx = ctx
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

		log.Printf("[❗️] Failed to compile regular expression : %s\n", err.Error())
		return false

	}

	return reg.MatchString(address)

}

// Checks whether received string is valid txHash or not
func checkHash(hash string) bool {

	reg, err := regexp.Compile(`^(0x\w{64})$`)
	if err != nil {

		log.Printf("[❗️] Failed to compile regular expression : %s\n", err.Error())
		return false

	}

	return reg.MatchString(hash)

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

// SubscribeToPendingTxExit - Subscribe to topic where pending tx(s), getting
// confirmed are published
func SubscribeToPendingTxExit(ctx context.Context) (*redis.PubSub, error) {

	return SubscribeToTopic(ctx, config.GetPendingTxExitPublishTopic())

}

// SubscribeToQueuedTxExit - Subscribe to topic where queued tx(s), getting
// unstuck are published
func SubscribeToQueuedTxExit(ctx context.Context) (*redis.PubSub, error) {

	return SubscribeToTopic(ctx, config.GetQueuedTxExitPublishTopic())

}

// ListenToMessages - Attempts to listen to messages being published
// on topic to which graphQL client has subscribed to over websocket transport
//
// This can be run as a seperate go routine
//
// Before publishing any message to channel, on which graphQL client is listening,
// one publishing criteria check to be performed, which must return `true` for this
// tx to be eligible for publishing to client
//
// You can always blindly return `true` in your `evaluationCriteria` function,
// so that you get to receive any tx being published on topic of your interest
func ListenToMessages(ctx context.Context, pubsub *redis.PubSub, topics []string, comm chan<- *model.MemPoolTx, pubCriteria PublishingCriteria, params ...interface{}) {

	defer func() {
		close(comm)
	}()

	if !(topics != nil && len(topics) > 0) {

		log.Printf("[❗️] Empty topic list was unexpected\n")
		return

	}

	{
	OUTER:
		for {

			select {

			case <-parentCtx.Done():

				// Denotes `harmony` is being shutdown
				//
				// We must unsubscribe from all topics & get out of this infinite loop
				ctx := context.Background()

				for _, topic := range topics {
					UnsubscribeFromTopic(ctx, pubsub, topic)
				}

				break OUTER

			case <-ctx.Done():

				// Denotes client is not active anymore
				//
				// We must unsubscribe from all topics & get out of this infinite loop
				ctx := context.Background()

				for _, topic := range topics {
					UnsubscribeFromTopic(ctx, pubsub, topic)
				}

				break OUTER

			case <-time.After(time.Millisecond * time.Duration(300)):

				// If client is still active, we'll reach here in
				// 300 ms & continue to read message published, if any

				msg, err := pubsub.ReceiveTimeout(ctx, time.Microsecond*time.Duration(100))
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
					if message != nil && pubCriteria(message, params...) {
						comm <- message.ToGraphQL()
					}

				default:
					// @note Doing nothing yet

				}

			}

		}
	}

}

// UnsubscribeFromTopic - Given topic name to which client is already subscribed to,
// attempts to unsubscribe from
func UnsubscribeFromTopic(ctx context.Context, pubsub *redis.PubSub, topic string) {

	if err := pubsub.Unsubscribe(ctx, topic); err != nil {

		log.Printf("[❗️] Failed to unsubscribe from Redis pubsub : %s\n", err.Error())

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
