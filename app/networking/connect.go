package networking

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

// ConnectionManager - All connected peers to be kept track of, so that we don't attempt
// to reconnect to same peer again
type ConnectionManager struct {
	Lock            sync.RWMutex
	Peers           map[peer.ID]bool
	NewPeerChan     chan peer.ID
	DroppedPeerChan chan peer.ID
}

// IsConnected - Before attempting to (re-)establish connection
// with peer, check whether already connected or not
func (c *ConnectionManager) IsConnected(peerId peer.ID) bool {

	c.Lock.RLock()
	defer c.Lock.RUnlock()

	v, ok := c.Peers[peerId]
	if !ok {
		return false
	}

	return v

}

// Added - When new connection is established
func (c *ConnectionManager) Added(peerId peer.ID) {
	c.NewPeerChan <- peerId
}

// Dropped - When connection with some peer is dropped
func (c *ConnectionManager) Dropped(peerId peer.ID) {
	c.DroppedPeerChan <- peerId
}

// Start - Listen for new peer we're getting connected to/ dropped
//
// Also clean up list of dropped peers after every `n` time unit,
// to avoid lock contention as much as possible
func (c *ConnectionManager) Start(ctx context.Context) {

	for {
		select {

		case <-ctx.Done():
			return
		case peer := <-c.NewPeerChan:

			c.Peers[peer] = true

		case peer := <-c.DroppedPeerChan:

			c.Peers[peer] = false

		case <-time.After(time.Duration(100) * time.Millisecond):

			peers := make([]peer.ID, 0, len(c.Peers))

			for k, v := range c.Peers {
				if !v {
					peers = append(peers, k)
				}
			}

			c.Lock.Lock()

			for _, v := range peers {
				delete(c.Peers, v)
			}

			c.Lock.Unlock()

		}
	}

}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		Lock:            sync.RWMutex{},
		Peers:           make(map[peer.ID]bool),
		NewPeerChan:     make(chan peer.ID, 10),
		DroppedPeerChan: make(chan peer.ID, 10),
	}
}
