package networking

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"time"

	"github.com/itzmeanjan/harmony/app/config"
	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	libp2ptls "github.com/libp2p/go-libp2p-tls"
	ma "github.com/multiformats/go-multiaddr"
)

// CreateHost - Creates a libp2p host, to be used for communicating
// with other `harmony` peers
func CreateHost(ctx context.Context) (host.Host, error) {

	var source io.Reader

	if seed := config.GetNetworkingSeed(); seed != 0 {

		source = mrand.New(mrand.NewSource(int64(seed)))

	} else {

		source = crand.Reader

	}

	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, source)
	if err != nil {
		return nil, err
	}

	identity := libp2p.Identity(priv)

	addrs := libp2p.ListenAddrStrings([]string{
		fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", config.GetNetworkingPort()),
		fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/ws", config.GetNetworkingPort()),
	}...)

	security := libp2p.Security(libp2ptls.ID, libp2ptls.New)

	transports := libp2p.DefaultTransports

	connManager := libp2p.ConnectionManager(connmgr.NewConnManager(10, 100, time.Minute))

	opts := []libp2p.Option{identity, addrs, security, transports, connManager}

	return libp2p.New(ctx, opts...)

}

// ShowHost - Showing on which multi addresses given host is listening on
func ShowHost(_host host.Host) {

	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", _host.ID().Pretty()))

	for _, addr := range _host.Addrs() {

		log.Printf("📞 Listening on : %s\n", addr.Encapsulate(hostAddr))

	}

}
