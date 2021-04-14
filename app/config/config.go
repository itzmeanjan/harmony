package config

import (
	"log"
	"math"
	"runtime"
	"strconv"

	"github.com/spf13/viper"
)

// Read - Reading .env file content, during application start up
func Read(file string) error {
	viper.SetConfigFile(file)

	return viper.ReadInConfig()
}

// Get - Get config value by key
func Get(key string) string {
	return viper.GetString(key)
}

// GetUint - Parses value as uint64 & returns response
func GetUint(key string) uint64 {
	return viper.GetUint64(key)
}

// GetFloat - Parse confiig value as floating point number & return
func GetFloat(key string) float64 {
	return viper.GetFloat64(key)
}

// GetBool - Parses config value as boolean & returns
func GetBool(key string) bool {
	return viper.GetBool(key)
}

// GetMemPoolPollingPeriod - Read mempool polling period & attempt to
// parse it to string, where it's expected that this period will be
// provided in form of time duration with millisecond level precision
//
// Example: If you want to poll mempool content every 2 seconds, you must be
// writing 2000 in `.env` file
//
// If you don't provide any value for this expected field, by default it'll
// start using 1000ms i.e. after completion of this iteration, it'll sleep for
// 1000ms & again get to work
func GetMemPoolPollingPeriod() uint64 {

	if period := GetUint("MemPoolPollingPeriod"); period != 0 {
		return period
	}

	return 1000

}

// GetPendingPoolSize - Max #-of pending pool txs can be living in memory
func GetPendingPoolSize() uint64 {

	if size := GetUint("PendingPoolSize"); size != 0 {
		return size
	}

	return 1024

}

// GetQueuedPoolSize - Max #-of queued pool txs can be living in memory
func GetQueuedPoolSize() uint64 {

	if size := GetUint("QueuedPoolSize"); size != 0 {
		return size
	}

	return 1024

}

// GetPendingTxEntryPublishTopic - Read provided topic name from `.env` file
// where newly added pending pool tx(s) to be published
func GetPendingTxEntryPublishTopic() string {

	if v := Get("PendingTxEntryTopic"); len(v) != 0 {
		return v
	}

	log.Printf("[❗️] Failed to get topic for publishing new pending tx, using `pending_pool_entry`\n")
	return "pending_pool_entry"

}

// GetPendingTxExitPublishTopic - Read provided topic name from `.env` file
// where tx(s) removed from pending pool to be published
func GetPendingTxExitPublishTopic() string {

	if v := Get("PendingTxExitTopic"); len(v) != 0 {
		return v
	}

	log.Printf("[❗️] Failed to get topic for publishing tx removed from pending pool, using `pending_pool_exit`\n")
	return "pending_pool_exit"

}

// GetQueuedTxEntryPublishTopic - Read provided topic name from `.env` file
// where newly added queued pool tx(s) to be published
func GetQueuedTxEntryPublishTopic() string {

	if v := Get("QueuedTxEntryTopic"); len(v) != 0 {
		return v
	}

	log.Printf("[❗️] Failed to get topic for publishing new queued tx, using `queued_pool_entry`\n")
	return "queued_pool_entry"

}

// GetQueuedTxExitPublishTopic - Read provided topic name from `.env` file
// where tx(s) removed from queued pool to be published
func GetQueuedTxExitPublishTopic() string {

	if v := Get("QueuedTxExitTopic"); len(v) != 0 {
		return v
	}

	log.Printf("[❗️] Failed to get topic for publishing tx removed from queued pool, using `queued_pool_exit`\n")
	return "queued_pool_exit"

}

// GetRedisDBIndex - Read desired redis database index, which
// user asked `harmony` to use
//
// If nothing is provided, it'll use `1`, by default
func GetRedisDBIndex() uint8 {

	db := Get("RedisDB")

	_db, err := strconv.ParseUint(db, 10, 8)
	if err != nil {
		log.Printf("[❗️] Failed to parse redis database index : `%s`, using 1\n", err.Error())
		return 1
	}

	return uint8(_db)

}

// GetConcurrencyFactor - Size of worker pool, is dictated by rule below
//
// @note You can set floating point value for `ConcurrencyFactor` ( > 0 )
func GetConcurrencyFactor() int {

	f := int(math.Ceil(GetFloat("ConcurrencyFactor") * float64(runtime.NumCPU())))
	if f <= 0 {

		log.Printf("[❗️] Bad concurrency factor, using unit sized pool\n")
		return 1

	}

	return f

}

// GetPortNumber - Attempts to read user preferred port number
// for running harmony as a service, if failing/ port lesser than 1024
// uses default value `7000`
func GetPortNumber() uint64 {

	if port := GetUint("Port"); port > 1024 {
		return port
	}

	return 7000

}

// GetNetworkingPort - Libp2p service to be run on this port, used
// for communicating with peers over P2P network
func GetNetworkingPort() uint64 {

	if port := GetUint("NetworkingPort"); port > 1024 {
		return port
	}

	return 7001

}

// GetNetworkingStream - Libp2p stream name, to be for listening on this
// & also sending messages when communicating with peer
func GetNetworkingStream() string {

	if v := Get("NetworkingStream"); len(v) != 0 {
		return v
	}

	return "/harmony/v1.0.0"

}

// GetBootstrapPeer - Attempts to get user supplied bootstrap node identifier
// so that this node can connect to it
func GetBootstrapPeer() string {

	return Get("NetworkingBootstrap")

}

// GetNetworkingRendezvous - This is the string with which harmony nodes will advertise
// them with & this node will attempt to find other peers of same kind using this string
func GetNetworkingRendezvous() string {

	if v := Get("NetworkingRendezvous"); len(v) != 0 {
		return v
	}

	return "harmony"

}

// GetPeerDiscoveryMode - Kademlia DHT peer discovery mode
// 1 => Client mode
// 2 => Server mode ( This peer can act an rendezvous point )
func GetPeerDiscoveryMode() uint64 {

	if v := GetUint("NetworkingDiscoveryMode"); v > 0 && v < 3 {
		return v
	}

	// By default it's going to work as client i.e. won't help
	// others in discoverying peers, when some other node
	// attempts to use this node as rendezvous point
	return 1

}

// GetNetworkingChoice - Consider configuring value for this field
// so that you can express your desire for joining a larger pool
// of `harmony` nodes, to get a much broader view of mempool
//
// Consider putting `true`, if you're interested, otherwise ignore
func GetNetworkingChoice() bool {

	return GetBool("NetworkingEnabled")

}
