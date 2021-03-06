package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"
	"log"

	"github.com/itzmeanjan/harmony/app/data"
	"github.com/itzmeanjan/harmony/app/graph/generated"
	"github.com/itzmeanjan/harmony/app/graph/model"
)

var memPool *data.MemPool

// InitMemPool - Must be invoked when setting up
// application, so that graphql calls
// can query this mempool, for forming client responses
func InitMemPool(pool *data.MemPool) bool {

	if pool != nil {
		memPool = pool
		return true
	}

	log.Printf("[❌] Bad mempool received in graphQL handler\n")
	return false

}

func (r *queryResolver) Pending(ctx context.Context) ([]*model.MemPoolTx, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Queued(ctx context.Context) ([]*model.MemPoolTx, error) {
	panic(fmt.Errorf("not implemented"))
}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }
