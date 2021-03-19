package networking

import (
	"bufio"
	"context"
	"log"
	"time"

	"github.com/itzmeanjan/harmony/app/config"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

// ReadFrom - Read from stream
func ReadFrom(cancel context.CancelFunc, rw *bufio.ReadWriter) {

	defer cancel()

	for {

		// @note Delimiter needs to be defined
		data, err := rw.ReadBytes('\n')
		if err != nil {

			log.Printf("[❗️] Failed to read from stream : %s\n", err.Error())
			break

		}

		log.Printf("✅ Received from peer : %s\n", string(data))

	}

}

// WriteTo - Write to stream
func WriteTo(cancel context.CancelFunc, rw *bufio.ReadWriter) {

	defer cancel()

	for {

		// @note Do something meaningful
		_, err := rw.Write([]byte("harmony\n"))
		if err != nil {

			log.Printf("[❗️] Failed to write to stream : %s\n", err.Error())
			break

		}

		if err := rw.Flush(); err != nil {

			log.Printf("[❗️] Failed to flush stream write buffer : %s\n", err.Error())
			break
		}

	}

}

// Listen - Handle incoming connection of other harmony peer for certain supported
// protocol(s)
func Listen(_host host.Host) {

	_host.SetStreamHandler(protocol.ID(config.GetNetworkingStream()), func(stream network.Stream) {

		ctx, cancel := context.WithCancel(context.Background())
		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

		go ReadFrom(cancel, rw)
		go WriteTo(cancel, rw)

		log.Printf("🤩 Got new stream from peer\n")

		// @note This is a blocking call
		<-ctx.Done()
		<-time.After(time.Duration(100) * time.Millisecond)

		// Closing stream, may be it's already closed
		if err := stream.Close(); err != nil {

			log.Printf("[❗️] Failed to close stream : %s\n", err.Error())

		}

	})

}
