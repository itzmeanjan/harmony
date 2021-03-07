package graph

import (
	"errors"
	"log"
	"regexp"
	"time"

	"github.com/itzmeanjan/harmony/app/data"
	"github.com/itzmeanjan/harmony/app/graph/model"
)

var memPool *data.MemPool

// InitMemPool - Initializing mempool handle, in this module
// so that it can be used before responding back to graphql queries
func InitMemPool(pool *data.MemPool) error {

	if pool != nil {
		memPool = pool
		return nil
	}

	return errors.New("Bad mempool received in graphQL handler")

}

// Given a list of mempool tx(s), convert them to
// compatible list of graphql tx(s)
func toGraphQL(txs []*data.MemPoolTx) []*model.MemPoolTx {

	res := make([]*model.MemPoolTx, 0, len(txs))

	for _, tx := range txs {

		res = append(res, tx.ToGraphQL())

	}

	return res

}

// Attempts to parse duration, obtained from user query
func parseDuration(d string) (time.Duration, error) {

	dur, err := time.ParseDuration(d)
	if err != nil {
		return time.Duration(0), err
	}

	return dur, nil

}

// Checks whether received string is valid address or not
func checkAddress(address string) bool {

	reg, err := regexp.Compile(`^(0x\w{40})$`)
	if err != nil {

		log.Printf("[!] Failed to compile regular expression : %s\n", err.Error())
		return false

	}

	return reg.MatchString(address)

}
