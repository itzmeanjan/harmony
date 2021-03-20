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
		return nil, errors.New("invalid address")
	}

	return toGraphQL(memPool.PendingFrom(common.HexToAddress(addr))), nil
}

func (r *queryResolver) PendingTo(ctx context.Context, addr string) ([]*model.MemPoolTx, error) {
	if !checkAddress(addr) {
		return nil, errors.New("invalid address")
	}

	return toGraphQL(memPool.PendingTo(common.HexToAddress(addr))), nil
}

func (r *queryResolver) QueuedFrom(ctx context.Context, addr string) ([]*model.MemPoolTx, error) {
	if !checkAddress(addr) {
		return nil, errors.New("invalid address")
	}

	return toGraphQL(memPool.QueuedFrom(common.HexToAddress(addr))), nil
}

func (r *queryResolver) QueuedTo(ctx context.Context, addr string) ([]*model.MemPoolTx, error) {
	if !checkAddress(addr) {
		return nil, errors.New("invalid address")
	}

	return toGraphQL(memPool.QueuedTo(common.HexToAddress(addr))), nil
}

func (r *queryResolver) TopXPendingWithHighGasPrice(ctx context.Context, x int) ([]*model.MemPoolTx, error) {
	if x <= 0 {
		return nil, errors.New("bad argument")
	}

	return toGraphQL(memPool.TopXPendingWithHighGasPrice(uint64(x))), nil
}

func (r *queryResolver) TopXQueuedWithHighGasPrice(ctx context.Context, x int) ([]*model.MemPoolTx, error) {
	if x <= 0 {
		return nil, errors.New("bad argument")
	}

	return toGraphQL(memPool.TopXQueuedWithHighGasPrice(uint64(x))), nil
}

func (r *queryResolver) TopXPendingWithLowGasPrice(ctx context.Context, x int) ([]*model.MemPoolTx, error) {
	if x <= 0 {
		return nil, errors.New("bad argument")
	}

	return toGraphQL(memPool.TopXPendingWithLowGasPrice(uint64(x))), nil
}

func (r *queryResolver) TopXQueuedWithLowGasPrice(ctx context.Context, x int) ([]*model.MemPoolTx, error) {
	if x <= 0 {
		return nil, errors.New("bad argument")
	}

	return toGraphQL(memPool.TopXQueuedWithLowGasPrice(uint64(x))), nil
}

func (r *queryResolver) PendingDuplicates(ctx context.Context, hash string) ([]*model.MemPoolTx, error) {
	if !checkHash(hash) {
		return nil, errors.New("invalid txHash")
	}

	return toGraphQL(memPool.PendingDuplicates(common.HexToHash(hash))), nil
}

func (r *queryResolver) QueuedDuplicates(ctx context.Context, hash string) ([]*model.MemPoolTx, error) {
	if !checkHash(hash) {
		return nil, errors.New("invalid txHash")
	}

	return toGraphQL(memPool.QueuedDuplicates(common.HexToHash(hash))), nil
}

func (r *subscriptionResolver) NewPendingTx(ctx context.Context) (<-chan *model.MemPoolTx, error) {
	_pubsub, err := SubscribeToPendingTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetPendingTxEntryPublishTopic()}

	// Because client wants to listen to any tx being published on this topic
	go ListenToMessages(ctx, _pubsub, topics, comm, NoCriteria)

	return comm, nil
}

func (r *subscriptionResolver) NewQueuedTx(ctx context.Context) (<-chan *model.MemPoolTx, error) {
	_pubsub, err := SubscribeToQueuedTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetQueuedTxEntryPublishTopic()}

	// Because client wants to listen to any tx being published on this topic
	go ListenToMessages(ctx, _pubsub, topics, comm, NoCriteria)

	return comm, nil
}

func (r *subscriptionResolver) NewConfirmedTx(ctx context.Context) (<-chan *model.MemPoolTx, error) {
	_pubsub, err := SubscribeToPendingTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetPendingTxExitPublishTopic()}

	// Because client wants to listen to any tx being published on this topic
	go ListenToMessages(ctx, _pubsub, topics, comm, NoCriteria)

	return comm, nil
}

func (r *subscriptionResolver) NewUnstuckTx(ctx context.Context) (<-chan *model.MemPoolTx, error) {
	_pubsub, err := SubscribeToQueuedTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetQueuedTxExitPublishTopic()}

	// Because client wants to listen to any tx being published on this topic
	go ListenToMessages(ctx, _pubsub, topics, comm, NoCriteria)

	return comm, nil
}

func (r *subscriptionResolver) PendingPool(ctx context.Context) (<-chan *model.MemPoolTx, error) {
	_pubsub, err := SubscribeToPendingPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 2)
	topics := []string{config.GetPendingTxEntryPublishTopic(), config.GetPendingTxExitPublishTopic()}

	// Because client wants to listen to any tx being published on these two topic
	go ListenToMessages(ctx, _pubsub, topics, comm, NoCriteria)

	return comm, nil
}

func (r *subscriptionResolver) QueuedPool(ctx context.Context) (<-chan *model.MemPoolTx, error) {
	_pubsub, err := SubscribeToQueuedPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 2)
	topics := []string{config.GetQueuedTxEntryPublishTopic(), config.GetQueuedTxExitPublishTopic()}

	// Because client wants to listen to any tx being published on these two topic
	go ListenToMessages(ctx, _pubsub, topics, comm, NoCriteria)

	return comm, nil
}

func (r *subscriptionResolver) MemPool(ctx context.Context) (<-chan *model.MemPoolTx, error) {
	_pubsub, err := SubscribeToMemPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 4)
	topics := []string{config.GetQueuedTxEntryPublishTopic(), config.GetQueuedTxExitPublishTopic(), config.GetPendingTxEntryPublishTopic(), config.GetPendingTxExitPublishTopic()}

	// Because client wants to listen to any tx being published on these two topic
	go ListenToMessages(ctx, _pubsub, topics, comm, NoCriteria)

	return comm, nil
}

func (r *subscriptionResolver) NewPendingTxFrom(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToPendingTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetPendingTxEntryPublishTopic()}

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewQueuedTxFrom(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToQueuedTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetQueuedTxEntryPublishTopic()}

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewConfirmedTxFrom(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToPendingTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetPendingTxExitPublishTopic()}

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewUnstuckTxFrom(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToQueuedTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetQueuedTxExitPublishTopic()}

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewTxFromAInPendingPool(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToPendingPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 2)
	topics := []string{config.GetPendingTxEntryPublishTopic(), config.GetPendingTxExitPublishTopic()}

	// Because client wants to get notified only when tx from certain address is detected
	// to be entering/ leaving pending pool
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewTxFromAInQueuedPool(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToQueuedPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 2)
	topics := []string{config.GetQueuedTxEntryPublishTopic(), config.GetQueuedTxExitPublishTopic()}

	// Because client wants to get notified only when tx from certain address is detected
	// to be entering/ leaving queued pool
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewTxFromAInMemPool(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToMemPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 4)
	topics := []string{
		config.GetQueuedTxEntryPublishTopic(),
		config.GetQueuedTxExitPublishTopic(),
		config.GetPendingTxEntryPublishTopic(),
		config.GetPendingTxExitPublishTopic()}

	// Because client wants to get notified only when tx from certain address is detected
	// to be entering/ leaving mem pool
	//
	// @note Mempool includes both pending & queued pool
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckFromAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewPendingTxTo(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToPendingTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetPendingTxEntryPublishTopic()}

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckToAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewQueuedTxTo(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToQueuedTxEntry(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetQueuedTxEntryPublishTopic()}

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckToAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewConfirmedTxTo(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToPendingTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetPendingTxExitPublishTopic()}

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckToAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewUnstuckTxTo(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToQueuedTxExit(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 1)
	topics := []string{config.GetQueuedTxExitPublishTopic()}

	// Because client wants to get notified only when tx of certain address is detected
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckToAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewTxToAInPendingPool(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToPendingPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 2)
	topics := []string{config.GetPendingTxEntryPublishTopic(), config.GetPendingTxExitPublishTopic()}

	// Because client wants to get notified only when tx to certain address is detected
	// to be entering/ leaving pending pool
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckToAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewTxToAInQueuedPool(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToQueuedPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 2)
	topics := []string{config.GetQueuedTxEntryPublishTopic(), config.GetQueuedTxExitPublishTopic()}

	// Because client wants to get notified only when tx to certain address is detected
	// to be entering/ leaving queued pool
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckToAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) NewTxToAInMemPool(ctx context.Context, address string) (<-chan *model.MemPoolTx, error) {
	if !checkAddress(address) {
		return nil, errors.New("invalid address")
	}

	_pubsub, err := SubscribeToMemPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 4)
	topics := []string{
		config.GetQueuedTxEntryPublishTopic(),
		config.GetQueuedTxExitPublishTopic(),
		config.GetPendingTxEntryPublishTopic(),
		config.GetPendingTxExitPublishTopic()}

	// Because client wants to get notified only when tx to certain address is detected
	// to be entering/ leaving mem pool
	//
	// @note Mempool denotes both pending & queued pool
	go ListenToMessages(ctx, _pubsub, topics, comm, CheckToAddress, common.HexToAddress(address))

	return comm, nil
}

func (r *subscriptionResolver) WatchTx(ctx context.Context, hash string) (<-chan *model.MemPoolTx, error) {
	if !checkHash(hash) {
		return nil, errors.New("invalid txHash")
	}

	_pubsub, err := SubscribeToMemPool(ctx)
	if err != nil {
		return nil, err
	}

	comm := make(chan *model.MemPoolTx, 4)
	topics := []string{
		config.GetQueuedTxEntryPublishTopic(),
		config.GetQueuedTxExitPublishTopic(),
		config.GetPendingTxEntryPublishTopic(),
		config.GetPendingTxExitPublishTopic()}

	go ListenToMessages(ctx, _pubsub, topics, comm, LinkedTx, common.HexToHash(hash))

	return comm, nil
}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
