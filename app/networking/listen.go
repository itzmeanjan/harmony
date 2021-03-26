package networking

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/graph"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

// ReadFrom - Read from stream
func ReadFrom(cancel context.CancelFunc, rw *bufio.ReadWriter) {

	defer cancel()

	for {

		// @note Need to make use of data received from peer
		data, err := rw.ReadBytes('\n')
		if err != nil {

			if err == io.EOF {
				break
			}

			log.Printf("[❗️] Failed to read from stream : %s\n", err.Error())
			break

		}

		tx := graph.UnmarshalPubSubMessage(data)
		if tx == nil {

			log.Printf("[❗️] Failed to deserialise received message from peer\n")
			continue

		}

		log.Printf("✅ Received from peer : %d bytes\n", len(data))

	}

}

// WriteTo - Write to mempool changes into stream i.e. connection
// with some remote peer
func WriteTo(cancel context.CancelFunc, rw *bufio.ReadWriter) {

	// @note Deferred functions are executed in LIFO order
	defer cancel()

	ctx := context.Background()
	topics := []string{config.GetQueuedTxEntryPublishTopic(),
		config.GetQueuedTxExitPublishTopic(),
		config.GetPendingTxEntryPublishTopic(),
		config.GetPendingTxExitPublishTopic()}

	pubsub, err := graph.SubscribeToMemPool(ctx)
	if err != nil {

		log.Printf("[❗️] Failed to subscribe to mempool changes : %s\n", err.Error())
		return

	}

OUTER:
	for {

		<-time.After(time.Millisecond * time.Duration(1))

		msg, err := pubsub.ReceiveTimeout(ctx, time.Millisecond*time.Duration(9))
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
				return
			}

		case *redis.Message:

			_, err := rw.Write([]byte(fmt.Sprintf("%s\n", m.Payload)))
			if err != nil {

				log.Printf("[❗️] Failed to write to stream : %s\n", err.Error())
				break OUTER

			}

			if err := rw.Flush(); err != nil {

				log.Printf("[❗️] Failed to flush stream write buffer : %s\n", err.Error())
				break OUTER

			}

		default:
			// @note Doing nothing yet

		}

	}

	if err := pubsub.Unsubscribe(context.Background(), topics...); err != nil {
		log.Printf("[❗️] Failed to unsubscribe from Redis pubsub topic(s) : %s\n", err.Error())
	}

}

// HandleStream - Attepts new stream & handles it through out its life time
func HandleStream(stream network.Stream) {

	ctx, cancel := context.WithCancel(context.Background())
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go ReadFrom(cancel, rw)
	go WriteTo(cancel, rw)

	log.Printf("🤩 Got new stream from peer : %s\n", stream.Conn().RemoteMultiaddr())

	// @note This is a blocking call
	<-ctx.Done()

	// Closing stream, may be it's already closed
	if err := stream.Close(); err != nil {

		log.Printf("[❗️] Failed to close stream : %s\n", err.Error())

	}

}

// Listen - Handle incoming connection of other harmony peer for certain supported
// protocol(s)
func Listen(_host host.Host) {

	_host.SetStreamHandler(protocol.ID(config.GetNetworkingStream()), HandleStream)

}
