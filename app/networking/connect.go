package networking

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

// IsConnected - Queries can be sent to connection manager
// by worker go routines over channel & response to be sent
// back to them over `response` channel
type IsConnected struct {
	Peer     peer.ID
	Response chan bool
}

// ConnectionManager - All connected peers to be kept track of, so that we don't attempt
// to reconnect to same peer again
type ConnectionManager struct {
	Peers           map[peer.ID]bool
	NewPeerChan     chan peer.ID
	DroppedPeerChan chan peer.ID
	IsConnectedChan chan IsConnected
}

// Added - When new connection is established
func (c *ConnectionManager) Added(peerId peer.ID) {
	c.NewPeerChan <- peerId
}

// Dropped - When connection with some peer is dropped
func (c *ConnectionManager) Dropped(peerId peer.ID) {
	c.DroppedPeerChan <- peerId
}

// IsConnected - Before attempting to (re-)establish connection
// with peer, check whether already connected or not
func (c *ConnectionManager) IsConnected(peerId peer.ID) bool {

	responseChan := make(chan bool)
	c.IsConnectedChan <- IsConnected{Peer: peerId, Response: responseChan}

	// This is a blocking call
	return <-responseChan

}

// Start - Listen for new peer we're getting connected to/ dropped
//
// Also clean up list of dropped peers after every `n` time unit,
// to avoid lock contention as much as possible
func (c *ConnectionManager) Start(ctx context.Context) {

	for {
		select {

		case <-ctx.Done():
			// Just stop doing what you're doing
			return

		case peer := <-c.NewPeerChan:

			c.Peers[peer] = true

		case peer := <-c.DroppedPeerChan:

			c.Peers[peer] = false

		case query := <-c.IsConnectedChan:
			// When worker go routines i.e. managing interaction
			// with remote peers, asks connection manager whether we've
			// already an working stream established with a remote peer
			// or not, it'll be received over this channel & responded back
			// over provided response channel
			//
			// Client is expected to listen on this channel for response

			v, ok := c.Peers[query.Peer]

			switch ok {

			case true:
				query.Response <- v
			case false:
				query.Response <- false

			}

		case <-time.After(time.Duration(10) * time.Millisecond):

			peers := make([]peer.ID, 0, len(c.Peers))

			for k, v := range c.Peers {
				if !v {
					peers = append(peers, k)
				}
			}

			for _, v := range peers {
				delete(c.Peers, v)
			}

		}
	}

}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		Peers:           make(map[peer.ID]bool),
		NewPeerChan:     make(chan peer.ID, 100),
		DroppedPeerChan: make(chan peer.ID, 100),
		IsConnectedChan: make(chan IsConnected, 100),
	}
}
