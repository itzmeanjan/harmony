package networking

import (
	"bufio"
	"context"
	"log"

	"github.com/itzmeanjan/harmony/app/config"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

// ReadFrom - ...
func ReadFrom(ctx context.Context, rw *bufio.ReadWriter) {

}

// WriteTo - ...
func WriteTo(ctx context.Context, rw *bufio.ReadWriter) {

}

// Listen - Handle incoming connection of other harmony peer for certain supported
// protocol(s)
func Listen(_host host.Host) {

	_host.SetStreamHandler(protocol.ID(config.GetNetworkingStream()), func(stream network.Stream) {

		ctx, cancel := context.WithCancel(context.Background())
		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

		go ReadFrom(ctx, rw)
		go WriteTo(ctx, rw)

		log.Printf("🤩 Got new stream from peer\n")

		// @note This is a blocking call
		<-ctx.Done()

		// Letting both reader/ writer know, they must stop working
		// as they run is different go routine
		cancel()
		// Closing stream, may be it's already closed
		if err := stream.Close(); err != nil {

			log.Printf("[❗️] Failed to close stream : %s\n", err.Error())

		}

	})

}
