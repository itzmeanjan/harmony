package networking

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

// ReconnectionManager - When stream to peer gets reset, it can be attempted
// to be reestablished, by sending request to this worker
//
// All peer related information required for doing so, will be kept in this struct
type ReconnectionManager struct {
	Host        host.Host
	Peers       map[peer.ID]time.Time
	WaitingTime time.Duration
	NewPeerChan chan peer.ID
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

// NewReconnectionManager - ...
func NewReconnectionManager() *ReconnectionManager {
	return &ReconnectionManager{
		Peers:       make(map[peer.ID]time.Time),
		WaitingTime: time.Duration(1) * time.Minute,
	}
}
