package networking

import "context"

// Setup - Bootstraps `harmony`'s p2p networking side
func Setup(ctx context.Context) error {

	// Attempt to create a new `harmony` node
	// with p2p networking capabilities
	host, err := CreateHost(ctx)
	if err != nil {
		return err
	}

	// Display info regarding this node
	ShowHost(host)
	// Start listening for incoming streams, for supported protocol
	Listen(host)

	go SetUpPeerDiscovery(ctx, host)

	return nil

}
