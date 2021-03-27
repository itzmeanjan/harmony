package networking

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/itzmeanjan/harmony/app/config"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

// AttemptDetails - Keeps track of how many times we attempted to
// connected to certain peer
//
// These are to be refreshed after each failed attempt
type AttemptDetails struct {
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

// FailedAttempt - Attempt to connect to some peer just failed, we're
// updating details, so that when to attempt next can be determined easily
func (a *AttemptDetails) FailedAttempt() {

	a.LastAttemptedAt = time.Now().UTC()
	a.Times++

	if a.Times > 1 {

		dur := a.Delay.Seconds()

		// Time span, to wait for ( from now ), before
		// next time it can be attempted, being
		// calculated in using fibonacci sequence
		//
		// This waiting time can be at max 3599s
		next := uint64(math.Round(dur*(1+math.Sqrt(5))/2.0)) % 3600

		a.Delay = time.Duration(next) * time.Second

	}

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
					Delay: time.Duration(5) * time.Second,
				}

			}

		case <-time.After(time.Duration(100) * time.Millisecond):
			// Every 100ms, attempting to connect to peers

			success := make([]peer.ID, 0, len(r.Peers))

			for k, v := range r.Peers {

				if !v.ShallWeAttempt(r.MaxAttemptCount) {
					continue
				}

				stream, err := r.Host.NewStream(ctx, k, protocol.ID(config.GetNetworkingStream()))
				if err != nil {

					log.Printf("[❗️] Peer reconnection attempt failed : %s\n", k)

					// Marking this connection attempt failed, which will
					// help it in preparing when to attempt next
					v.FailedAttempt()
					continue

				}

				func(stream network.Stream) {
					go HandleStream(stream)
				}(stream)

				success = append(success, k)
				log.Printf("✅ Reconnected to dropped peer : %s\n", k)

			}

			// Dropping entries of those peers, with whom
			// we've successfully established connection
			for _, v := range success {
				delete(r.Peers, v)
			}

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
