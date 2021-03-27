package networking

import (
	"context"
	"time"

	"github.com/itzmeanjan/harmony/app/config"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

// AttemptDetails - Keeps track of how many times we attempted to
// connected to certain peer
//
// These are to be refreshed after each failed attempt
type AttemptDetails struct {
	StartedAt       time.Time
	LastAttemptedAt time.Time
	Times           uint64
	Delay           time.Duration
}

// ReconnectionManager - When stream to peer gets reset, it can be attempted
// to be reestablished, by sending request to this worker
//
// All peer related information required for doing so, will be kept in this struct
type ReconnectionManager struct {
	Host            host.Host
	Peers           map[peer.ID]*AttemptDetails
	MaxAttemptCount uint64
	NewPeerChan     chan peer.ID
}

// Start - ...
func (r *ReconnectionManager) Start(ctx context.Context) {

	for {

		select {

		case <-ctx.Done():
			return
		case <-time.After(time.Duration(100) * time.Millisecond):

		}

	}

}

// NewReconnectionManager - Creates a new instance of peer reconnection
// attempt manager
func NewReconnectionManager(_host host.Host) *ReconnectionManager {
	return &ReconnectionManager{
		Host:            _host,
		Peers:           make(map[peer.ID]*AttemptDetails),
		MaxAttemptCount: config.GetMaxReconnectionAttemptCount(),
		NewPeerChan:     make(chan peer.ID, 10),
	}
}
