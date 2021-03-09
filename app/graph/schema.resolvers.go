package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/itzmeanjan/harmony/app/config"
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

func (r *queryResolver) TopXPendingWithHighGasPrice(ctx context.Context, x int) ([]*model.MemPoolTx, error) {

	if x <= 0 {
		return nil, errors.New("Bad argument")
	}

	return toGraphQL(memPool.TopXPendingWithHighGasPrice(uint64(x))), nil

}

func (r *queryResolver) TopXQueuedWithHighGasPrice(ctx context.Context, x int) ([]*model.MemPoolTx, error) {

	if x <= 0 {
		return nil, errors.New("Bad argument")
	}

	return toGraphQL(memPool.TopXQueuedWithHighGasPrice(uint64(x))), nil

}

func (r *queryResolver) TopXPendingWithLowGasPrice(ctx context.Context, x int) ([]*model.MemPoolTx, error) {

	if x <= 0 {
		return nil, errors.New("Bad argument")
	}

	return toGraphQL(memPool.TopXPendingWithLowGasPrice(uint64(x))), nil

}

func (r *queryResolver) TopXQueuedWithLowGasPrice(ctx context.Context, x int) ([]*model.MemPoolTx, error) {

	if x <= 0 {
		return nil, errors.New("Bad argument")
	}

	return toGraphQL(memPool.TopXQueuedWithLowGasPrice(uint64(x))), nil

}

func (r *subscriptionResolver) NewPendingTx(ctx context.Context) (<-chan *model.MemPoolTx, error) {

	_pubsub, err := SubscribeToPendingTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)

	// Because client wants to listen to any tx being published on this topic
	go ListenToMessages(ctx, _pubsub, config.GetPendingTxEntryPublishTopic(), comm, NoCriteria)

	return comm, nil

}

func (r *subscriptionResolver) NewQueuedTx(ctx context.Context) (<-chan *model.MemPoolTx, error) {

	_pubsub, err := SubscribeToQueuedTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)

	// Because client wants to listen to any tx being published on this topic
	go ListenToMessages(ctx, _pubsub, config.GetQueuedTxEntryPublishTopic(), comm, NoCriteria)

	return comm, nil

}

func (r *subscriptionResolver) NewConfirmedTx(ctx context.Context) (<-chan *model.MemPoolTx, error) {

	_pubsub, err := SubscribeToPendingTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)

	// Because client wants to listen to any tx being published on this topic
	go ListenToMessages(ctx, _pubsub, config.GetPendingTxExitPublishTopic(), comm, NoCriteria)

	return comm, nil

}

func (r *subscriptionResolver) NewUnstuckTx(ctx context.Context) (<-chan *model.MemPoolTx, error) {

	_pubsub, err := SubscribeToQueuedTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)

	// Because client wants to listen to any tx being published on this topic
	go ListenToMessages(ctx, _pubsub, config.GetQueuedTxExitPublishTopic(), comm, NoCriteria)

	return comm, nil

}

func (r *subscriptionResolver) NewPendingTxFrom(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {

	if !checkAddress(address) {
		return nil, errors.New("Invalid address")
	}

	_pubsub, err := SubscribeToPendingTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, config.GetPendingTxEntryPublishTopic(), comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil

}

func (r *subscriptionResolver) NewQueuedTxFrom(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {

	if !checkAddress(address) {
		return nil, errors.New("Invalid address")
	}

	_pubsub, err := SubscribeToQueuedTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, config.GetQueuedTxEntryPublishTopic(), comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil

}

func (r *subscriptionResolver) NewConfirmedTxFrom(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {

	if !checkAddress(address) {
		return nil, errors.New("Invalid address")
	}

	_pubsub, err := SubscribeToPendingTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, config.GetPendingTxExitPublishTopic(), comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil

}

func (r *subscriptionResolver) NewUnstuckTxFrom(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {

	if !checkAddress(address) {
		return nil, errors.New("Invalid address")
	}

	_pubsub, err := SubscribeToQueuedTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, config.GetQueuedTxExitPublishTopic(), comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil

}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
