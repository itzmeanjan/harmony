package graph

import (
	"errors"

	"github.com/itzmeanjan/harmony/app/data"
)

var memPool *data.MemPool

// InitMemPool - Initializing mempool handle, in this module
// so that it can be used before responding back to graphql queries
func InitMemPool(pool *data.MemPool) error {

	if pool != nil {
		memPool = pool
		return nil
	}

	return errors.New("Bad mempool received in graphQL handler")

}
