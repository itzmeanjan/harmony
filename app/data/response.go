package data

// Stat - Response to client queries for current mempool state
// to be sent in this form
type Stat struct {
	PendingPoolSize uint64 `json:"pendingPoolSize"`
	QueuedPoolSize  uint64 `json:"queuedPoolSize"`
	Uptime          string `json:"uptime"`
	NetworkID       uint64 `json:"networkID"`
}
