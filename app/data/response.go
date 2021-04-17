package data

import "time"

// Stat - Response to client queries for current mempool state
// to be sent in this form
type Stat struct {
	PendingPoolSize uint64        `json:"pendingPoolSize"`
	QueuedPoolSize  uint64        `json:"queuedPoolSize"`
	Uptime          string        `json:"uptime"`
	Processed       uint64        `json:"processed"`
	LatestBlock     uint64        `json:"latestBlock"`
	SeenAgo         time.Duration `json:"latestSeenAgo"`
	NetworkID       uint64        `json:"networkID"`
}

// Msg - Response message sent to client
type Msg struct {
	Code    uint8  `json:"code,omitempty"`
	Message string `json:"message"`
}
