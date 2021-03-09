package graph

import "github.com/itzmeanjan/harmony/app/data"

// PublishingCriteria - Message publishing criteria is expected
// in this form, which is to be invoked, everytime new message
// is received in any topic client is subscribed to
type PublishingCriteria func(*data.MemPoolTx, ...interface{}) bool

// NoCriteria - When you want to listen to
// any tx being published on your topic of interest
// simply pass this function to `ListenToMessages`
// so that all criteria check always returns `true`
// & graphQL client receives all tx(s)
func NoCriteria(*data.MemPoolTx, ...interface{}) bool {
	return true
}
