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

func (r *queryResolver) Pending(ctx context.Context) ([]*model.MemPoolTx, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Queued(ctx context.Context) ([]*model.MemPoolTx, error) {
	panic(fmt.Errorf("not implemented"))
}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
var memPool *data.MemPool

func InitMemPool(pool *data.MemPool) bool {

	if pool != nil {
		memPool = pool
		return true
	}

	log.Printf("[❌] Bad mempool received in graphQL handler\n")
	return false

}
