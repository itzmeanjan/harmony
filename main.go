package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/itzmeanjan/harmony/app/bootup"
)

func main() {

	log.Printf("[😌] Harmony - Reducing Chaos in MemPool\n")

	abs, err := filepath.Abs(".env")
	if err != nil {

		log.Printf("[❗️] Failed to find absolute path of file : %s\n", err.Error())
		os.Exit(1)

	}

	// This is application's root level context, to be passed down
	// to worker go routines
	ctx, cancel := context.WithCancel(context.TODO())

	resources, err := bootup.SetGround(ctx, abs)
	if err != nil {

		log.Printf("[❗️] Failed to acquire resource(s) : %s\n", err.Error())
		os.Exit(1)

	}

	// Attempt to catch interrupt event(s)
	// so that graceful shutdown can be performed
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

	go func() {

		// To be invoked when returning from this
		// go rountine's execution scope
		defer func() {

			resources.Release()

			// Stopping process
			log.Printf("\n[✅] Gracefully shut down `harmony`\n")
			os.Exit(0)

		}()

		// Attempting to catch OS interrupt
		// sent to `harmony`
		<-interruptChan

		// When interrupt is received, attempting to
		// let all other go routines know, master go routine
		// wants all to shut down, they must do a graceful shut down
		// of what they're doing now
		cancel()

		// Giving workers 3 seconds, before forcing shutdown
		//
		// This is simply a blocking call i.e. blocks for 3 seconds
		<-time.After(time.Second * time.Duration(3))

	}()

}
