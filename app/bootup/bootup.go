package bootup

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/data"
)

// SetGround - This is to be called when starting application
// for doing basic ground work(s), so that all required resources
// are available for further usage during application lifetime
func SetGround(ctx context.Context, file string) (*data.Resource, error) {

	if err := config.Read(file); err != nil {
		return nil, err
	}

	client, err := ethclient.DialContext(ctx, config.Get("RPCUrl"))

	if err != nil {
		return nil, err
	}

	return &data.Resource{RPCClient: client}, nil

}
