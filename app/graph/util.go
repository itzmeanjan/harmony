package graph

import (
	"context"
	"errors"
	"log"
	"regexp"
	"time"

	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/data"
	"github.com/itzmeanjan/harmony/app/graph/model"
	"github.com/itzmeanjan/pubsub"
)

var memPool *data.MemPool
var pubsubHub *pubsub.PubSub
var parentCtx context.Context

// InitMemPool - Initializing mempool handle, in this module
// so that it can be used before responding back to graphql queries
func InitMemPool(pool *data.MemPool) error {

	if pool != nil {
		memPool = pool
		return nil
	}

	return errors.New("bad mempool received in graphQL handler")

}

// InitPubSub - Initializing pubsub handle, for managing subscriptions
func InitPubSub(client *pubsub.PubSub) error {

	if client != nil {
		pubsubHub = client
		return nil
	}

	return errors.New("bad pub/sub received in graphQL handler")

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

	for i := 0; i < len(txs); i++ {
		res = append(res, txs[i].ToGraphQL())
	}

	data.CleanSlice(txs)
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

// SubscribeToTopic - Subscribes to PubSub topic(s), while configuring subscription such
// that at max 256 messages can be kept in buffer at a time. If client is consuming slowly
// it might miss some messages when buffer stays full.
func SubscribeToTopic(ctx context.Context, topic ...string) (*pubsub.Subscriber, error) {

	_sub := pubsubHub.Subscribe(256, topic...)
	if _sub == nil {
		return nil, errors.New("topic subscription failed")
	}

	return _sub, nil

}

// SubscribeToPendingPool - Subscribes to both topics, associated with changes
// happening in pending tx pool
//
// When tx joins/ leaves pending pool, subscribers will receive notification
func SubscribeToPendingPool(ctx context.Context) (*pubsub.Subscriber, error) {

	return SubscribeToTopic(ctx, config.GetPendingTxEntryPublishTopic(), config.GetPendingTxExitPublishTopic())

}

// SubscribeToQueuedPool - Subscribes to both topics, associated with changes
// happening in queued tx pool
//
// When tx joins/ leaves queued pool, subscribers will receive notification
//
// @note Tx(s) generally join queued pool, when there's nonce gap & this tx can't be
// processed until some lower nonce tx(s) get(s) processed
func SubscribeToQueuedPool(ctx context.Context) (*pubsub.Subscriber, error) {

	return SubscribeToTopic(ctx, config.GetQueuedTxEntryPublishTopic(), config.GetQueuedTxExitPublishTopic())

}

// SubscribeToMemPool - Subscribes to any changes happening in mempool
//
// As mempool has two segments i.e.
// 1. Pending Pool
// 2. Queued Pool
//
// It'll subscribe to all 4 topics for listening
// to tx(s) entering/ leaving any portion of mempool
func SubscribeToMemPool(ctx context.Context) (*pubsub.Subscriber, error) {

	return SubscribeToTopic(ctx,
		config.GetQueuedTxEntryPublishTopic(),
		config.GetQueuedTxExitPublishTopic(),
		config.GetPendingTxEntryPublishTopic(),
		config.GetPendingTxExitPublishTopic())

}

// SubscribeToPendingTxEntry - Subscribe to topic where new pending tx(s)
// are published
func SubscribeToPendingTxEntry(ctx context.Context) (*pubsub.Subscriber, error) {

	return SubscribeToTopic(ctx, config.GetPendingTxEntryPublishTopic())

}

// SubscribeToQueuedTxEntry - Subscribe to topic where new queued tx(s)
// are published
func SubscribeToQueuedTxEntry(ctx context.Context) (*pubsub.Subscriber, error) {

	return SubscribeToTopic(ctx, config.GetQueuedTxEntryPublishTopic())

}

// SubscribeToPendingTxExit - Subscribe to topic where pending tx(s), getting
// confirmed are published
func SubscribeToPendingTxExit(ctx context.Context) (*pubsub.Subscriber, error) {

	return SubscribeToTopic(ctx, config.GetPendingTxExitPublishTopic())

}

// SubscribeToQueuedTxExit - Subscribe to topic where queued tx(s), getting
// unstuck are published
func SubscribeToQueuedTxExit(ctx context.Context) (*pubsub.Subscriber, error) {

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
func ListenToMessages(ctx context.Context, subscriber *pubsub.Subscriber, topics []string, comm chan<- *model.MemPoolTx, pubCriteria PublishingCriteria, params ...interface{}) {

	defer func() {
		close(comm)
	}()

	if !(len(topics) > 0) {

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
				UnsubscribeFromTopic(context.Background(), subscriber, topics...)

				break OUTER

			case <-ctx.Done():

				// Denotes client is not active anymore
				//
				// We must unsubscribe from all topics & get out of this infinite loop
				UnsubscribeFromTopic(context.Background(), subscriber, topics...)

				break OUTER

			default:

				// Read next message
				received := subscriber.Next()
				if received == nil {
					break
				}

				// New message has been published on topic
				// of our interest, we'll attempt to deserialize
				// data to deliver it to client in expected format
				message := UnmarshalPubSubMessage(received.Data)
				if message != nil && pubCriteria(message, params...) {

					// Only publish non-nil data i.e. if (de)-serialisation
					// fails some how, it's better to send nothing, rather than
					// sending client `nil`
					sendable := message.ToGraphQL()
					if sendable != nil {
						comm <- sendable
					}

				}

			}

		}
	}

}

// UnsubscribeFromTopic - Given topic name to which client is already subscribed to,
// attempts to unsubscribe from
func UnsubscribeFromTopic(ctx context.Context, subscriber *pubsub.Subscriber, topic ...string) {

	if ok, _ := subscriber.UnsubscribeAll(pubsubHub); !ok {
		log.Printf("[❗️] Failed to unsubscribe from topic(s)\n")
	}

	if !subscriber.Close() {
		log.Printf("[❗️] Failed to destroy subscriber\n")
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
