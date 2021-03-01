package config

import (
	"log"
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

	period := Get("MemPoolPollingPeriod")

	_period, err := strconv.ParseUint(period, 10, 64)
	if err != nil {
		log.Printf("[❗️] Failed to parse mempool polling period : `%s`, using 1000 ms\n", err.Error())
		return 1000
	}

	return _period

}
