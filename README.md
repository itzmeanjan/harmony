# harmony
Reduce Chaos in MemPool 😌

![banner](./sc/banner.png)

## Motivation

I discovered **Ethereum's MemPool is one of the least explored domains, but not really least important**. 

Whenever a block is mined & some tx(s) are included in it, it's pretty much, _value living at rest_, whereas is case of mempool, _value is in-flight_. A lot of tx(s) are fighting for their space in next block to be mined, where only a few will get their place. But who will get, based or what criteria, it's not very much well defined.

We generally believe giving higher gas price compared to other tx(s) currently present in mempool, gives better chance that block miner will pick this tx during next block mining. But block miner always gets to override that, using custom logic. Also any one can write an automated program for monitoring mempool and replicate tx(s) of their interest with their own address, higher gas price - which may help them in cutting deal faster than original one or benefitting if target smart contract has some security loophole.

During my journey of exploring Ethereum MemPool, I found good initiative from [BlockNative](https://www.blocknative.com) in demystifying MemPool. They've built some interesting products on top of mempool.

`harmony - Reduce Chaos in MemPool 😌`, aims to become a reliable mempool monitoring engine, while exposing useful functionalities for letting client applications write their monitoring logic seamlessly, with out worrying about underlying details too much 😎

- You can subscribe to listen for tx(s) going to/ from address of interest
- You can catch duplicate nonce tx(s), which of them gets accepted/ dropped
- You can build notification service on top of it
- It will help you in getting better gas price prediction
- It can be used for building real-time charts showing current network traffic
- Many more ...

## Prerequisite

- Make sure you've _`Go ( >= 1.16)`_, _`make`_ installed
- You need to also have _`Redis ( >= 5.x )`_

> Note : Consider setting up Redis instance with password protection

- Get one Ethereum Node up & running, with `txpool` RPC API enabled. You can always use public RPC nodes

## Installation

- For using `harmony`, let's first clone this repo

```bash
git clone https://github.com/itzmeanjan/harmony.git
```

- After getting inside `harmony`, create `.env` file with following content

```bash
cd harmony
touch .env
```

```bash
RPCUrl=https://<rpc-node>
MemPoolPollingPeriod=1000
PendingTxEntryTopic=pending_pool_entry
PendingTxExitTopic=pending_pool_exit
QueuedTxEntryTopic=queued_pool_entry
QueuedTxExitTopic=queued_pool_exit
RedisConnection=tcp
RedisAddress=127.0.0.1:6379
RedisPassword=redis
RedisDB=1
ConcurrencyFactor=10
Port=7000
```

Environment Variable | Interpretation
--- | ---
RPCUrl | `txpool` RPC API enabled Ethereum Node's URI
MemPoolPollingPeriod | RPC node's mempool to be checked every `X` milliseconds
PendingTxEntryTopic | Whenever tx enters pending pool, it'll be published on Redis topic `t`
PendingTxExitTopic | Whenever tx leaves pending pool, it'll be published on Redis topic `t`
QueuedTxEntryTopic | Whenever tx enters queued pool, it'll be published on Redis topic `t`
QueuedTxExitTopic | Whenever tx leaves queued pool, it'll be published on Redis topic `t`
RedisConnection | Communicate with Redis over transport protocol
RedisAddress | `address:port` combination of Redis
RedisPassword | Authentication details for talking to Redis. **[ Not mandatory ]**
RedisDB | Redis database to be used. **[ By default there're 16 of them ]**
ConcurrencyFactor | Whenever concurrency can be leveraged, `harmony` will create worker pool with `#-of logical CPUs x ConcurrencyFactor` go routines. **[ Can be float too ]**
Port | Starts HTTP server on this port ( > 1024 )

- Let's build & run `harmony`

```bash
make run
```
