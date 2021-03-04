![banner](./sc/banner.png)

# harmony
Reduce Chaos in MemPool 😌

## Motivation

I discovered Ethereum's MemPool is one of the least explored domains, but not really least important. Because whenever a block is mined & some tx(s) are included in it, it's pretty much, _value living at rest_. Whereas is case of mempool, value is in-flight. A lot of tx(s) are fighting for their space in next block to be mined, where only a few will get their place. But who will get, based or what criteria, it's not very much well defined.

We generally believe giving higher gas price compared to other tx(s) currently present in mempool, gives better chance that block miner will pick this tx during next block mining. But block miner always gets to override that, using custom logic. Also any one can write an automated program for monitoring mempool and replicate tx(s) of their interest with their own address, higher gas price - which will help them in benefitting if target smart contract has some loophole.

During my journey of exploring Ethereum MemPool, I found good initiative from [BlockNative](https://www.blocknative.com) in demystifying MemPool. They've built some interesting products on top of mempool.

Objective of `harmony` is to create an open implementation which can be deployed by anyone for looking inside MemPool & catch any event(s) they want to listen to. 

- You can subscribe to listen for tx(s) going to/ from address of interest
- You can catch duplicate nonce tx(s), which of them gets accepted/ dropped
- You can build notification service on top of it, to let someone know when they're going to receive something
- It will help you in getting better gas price prediction
- It can be used for building real-time charts showing network traffic
- Many more ...

> Note: Currently `harmony` is in its primary stage of development. More info on usage, coming soon.
