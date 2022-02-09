package utils

import (
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	registeredChannels []chan bool
	chanMutex          sync.Mutex
)

func RegisterExitChannel(ch chan bool) {
	chanMutex.Lock()
	defer chanMutex.Unlock()
	registeredChannels = append(registeredChannels, ch)
}

func SetupElegantExit() {
	log.Debugf("entering SetupElegantExit")
	// handle elegant exit; create support for channels of go routines that want to exit cleanly
	registeredChannels = make([]chan bool, 0)
	exitSigChan := make(chan os.Signal, 1)
	log.Debugf("registered exit signal channel")
	signal.Notify(exitSigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// wait for exit signal; then stop all the other go functions
		sig := <-exitSigChan
		log.Debugf("received exit signal = %v", sig)
		chanMutex.Lock()
		defer chanMutex.Unlock()
		// exit signal received; stop other go functions
		for _, ch := range registeredChannels {
			ch <- true
		}
		log.Debugf("exiting SetupElegantExit go function")
	}()
	log.Debugf("exiting SetupElegantExit")
}
