package networking

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/graph"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multiaddr"
)

// ReadFrom - Read from stream & attempt to deserialize length prefixed
// tx data received from peer, which will be acted upon
func ReadFrom(ctx context.Context, cancel context.CancelFunc, rw *bufio.ReadWriter, remote multiaddr.Multiaddr) {

	defer cancel()

	for {

		buf := make([]byte, 4)

		if _, err := io.ReadFull(rw.Reader, buf); err != nil {

			if err == io.EOF {
				break
			}

			log.Printf("[❗️] Failed to read size of next chunk : %s | %s\n", err.Error(), remote)
			break

		}

		size := binary.LittleEndian.Uint32(buf)

		chunk := make([]byte, size)

		if _, err := io.ReadFull(rw.Reader, chunk); err != nil {

			if err == io.EOF {
				break
			}

			log.Printf("[❗️] Failed to read chunk from peer : %s | %s\n", err.Error(), remote)
			break

		}

		tx := graph.UnmarshalPubSubMessage(chunk)
		if tx == nil {

			log.Printf("[❗️] Failed to deserialise message from peer | %s\n", remote)
			continue

		}

		if memPool.HandleTxFromPeer(ctx, redisClient, tx) {

			log.Printf("✅ New tx from peer : %d bytes | %s\n", len(chunk), remote)
			continue

		}

		log.Printf("👍 Seen tx from peer : %d bytes | %s\n", len(chunk), remote)

	}

}

// WriteTo - Write to mempool changes into stream i.e. connection
// with some remote peer
func WriteTo(ctx context.Context, cancel context.CancelFunc, rw *bufio.ReadWriter, remote multiaddr.Multiaddr) {

	// @note Deferred functions are executed in LIFO order
	defer cancel()

	topics := []string{config.GetQueuedTxEntryPublishTopic(),
		config.GetQueuedTxExitPublishTopic(),
		config.GetPendingTxEntryPublishTopic(),
		config.GetPendingTxExitPublishTopic()}

	pubsub, err := graph.SubscribeToMemPool(ctx)
	if err != nil {

		log.Printf("[❗️] Failed to subscribe to mempool changes : %s\n", err.Error())
		return

	}

	{
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

				chunk := make([]byte, 4+len(m.Payload))

				binary.LittleEndian.PutUint32(chunk[:4], uint32(len(m.Payload)))
				n := copy(chunk[4:], []byte(m.Payload))

				if n != len(m.Payload) {

					log.Printf("[❗️] Failed to prepare chunk for peer | %s\n", remote)
					break

				}

				if _, err := rw.Write(chunk); err != nil {

					log.Printf("[❗️] Failed to write chunk on stream : %s | %s\n", err.Error(), remote)
					break OUTER

				}

				if err := rw.Flush(); err != nil {

					log.Printf("[❗️] Failed to flush stream buffer : %s | %s\n", err.Error(), remote)
					break OUTER

				}

			default:
				// @note Doing nothing yet

			}

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
	remote := stream.Conn().RemoteMultiaddr()

	go ReadFrom(ctx, cancel, rw, remote)
	go WriteTo(ctx, cancel, rw, remote)

	log.Printf("🤩 Got new stream from peer : %s\n", remote)

	// @note This is a blocking call
	<-ctx.Done()

	// Closing stream, may be it's already closed
	if err := stream.Close(); err != nil {

		log.Printf("[❗️] Failed to close stream : %s\n", err.Error())

	}

	log.Printf("🙂 Dropped peer connection : %s\n", remote)

}

// Listen - Handle incoming connection of other harmony peer for certain supported
// protocol(s)
func Listen(_host host.Host) {

	_host.SetStreamHandler(protocol.ID(config.GetNetworkingStream()), HandleStream)

}
