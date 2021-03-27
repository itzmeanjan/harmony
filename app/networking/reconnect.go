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

// EverAttempted - Because we consider `times` == 0, to be
// never attempted
func (a *AttemptDetails) EverAttempted() bool {
	return a.Times > 0
}

// ShouldWeDrop - If all allowed attempts exhausted, we should
// not reattempt, we hope that peer will find us/ we'll via
// rendezvous
func (a *AttemptDetails) ShouldWeDrop(maxAttempt uint64) bool {
	return a.Times > maxAttempt
}

// ShallWeAttempt - If never attempted, surely we can attempt
//
// If we've exhausted all attempts, we'll not reattempt, they're to
// be dropped
//
// If time we were supposed to be waiting after last attempted timestamp
// has passed by, we can attempt
func (a *AttemptDetails) ShallWeAttempt(maxAttempt uint64) bool {

	if !a.EverAttempted() {
		return true
	}

	if a.Times > maxAttempt {
		return false
	}

	return a.LastAttemptedAt.Add(a.Delay).Before(time.Now().UTC())

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
		case _peerId := <-r.NewPeerChan:
			// Listening for peers, with whom we lost
			// connection ( stream got closed, actually )
			// & we'll attempt to reestablish that 👇

			_, ok := r.Peers[_peerId]
			if !ok {

				r.Peers[_peerId] = &AttemptDetails{
					StartedAt: time.Now().UTC(),
					Delay:     time.Duration(5) * time.Second,
				}

			}

		case <-time.After(time.Duration(100) * time.Millisecond):

		case <-time.After(time.Duration(1000) * time.Millisecond):
			// Every 1 seconds, we'll attempt to check if any peer
			// we've enlisted, which are not required to be retried
			// anymore in future, so we can drop those

			eligibles := make([]peer.ID, 0, len(r.Peers))

			for k, v := range r.Peers {

				if v.ShouldWeDrop(r.MaxAttemptCount) {
					eligibles = append(eligibles, k)
				}

			}

			// If found any to be eligible, attempt to drop those
			for _, v := range eligibles {
				delete(r.Peers, v)
			}

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
