package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/itzmeanjan/harmony/app/graph/generated"
	"github.com/itzmeanjan/harmony/app/graph/model"
)

func (r *queryResolver) PendingForMoreThan(ctx context.Context, x string) ([]*model.MemPoolTx, error) {

	dur, err := parseDuration(x)
	if err != nil {
		return nil, err
	}

	return toGraphQL(memPool.PendingForGTE(dur)), nil

}

func (r *queryResolver) PendingForLessThan(ctx context.Context, x string) ([]*model.MemPoolTx, error) {

	dur, err := parseDuration(x)
	if err != nil {
		return nil, err
	}

	return toGraphQL(memPool.PendingForLTE(dur)), nil

}

func (r *queryResolver) QueuedForMoreThan(ctx context.Context, x string) ([]*model.MemPoolTx, error) {

	dur, err := parseDuration(x)
	if err != nil {
		return nil, err
	}

	return toGraphQL(memPool.QueuedForGTE(dur)), nil

}

func (r *queryResolver) QueuedForLessThan(ctx context.Context, x string) ([]*model.MemPoolTx, error) {

	dur, err := parseDuration(x)
	if err != nil {
		return nil, err
	}

	return toGraphQL(memPool.QueuedForLTE(dur)), nil

}

func (r *queryResolver) PendingFrom(ctx context.Context, addr string) ([]*model.MemPoolTx, error) {

	if !checkAddress(addr) {
		return nil, errors.New("Invalid address")
	}

	return toGraphQL(memPool.PendingFrom(common.HexToAddress(addr))), nil

}

func (r *queryResolver) PendingTo(ctx context.Context, addr string) ([]*model.MemPoolTx, error) {

	if !checkAddress(addr) {
		return nil, errors.New("Invalid address")
	}

	return toGraphQL(memPool.PendingTo(common.HexToAddress(addr))), nil

}

func (r *queryResolver) QueuedFrom(ctx context.Context, addr string) ([]*model.MemPoolTx, error) {

	if !checkAddress(addr) {
		return nil, errors.New("Invalid address")
	}

	return toGraphQL(memPool.QueuedFrom(common.HexToAddress(addr))), nil

}

func (r *queryResolver) QueuedTo(ctx context.Context, addr string) ([]*model.MemPoolTx, error) {

	if !checkAddress(addr) {
		return nil, errors.New("Invalid address")
	}

	return toGraphQL(memPool.QueuedTo(common.HexToAddress(addr))), nil

}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }
