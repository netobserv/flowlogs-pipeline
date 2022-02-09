package utils

import (
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

var (
	registeredChannels []chan bool
)

func RegisterExitChannel(ch chan bool) {
	registeredChannels = append(registeredChannels, ch)
}

func SetupElegantExit() {
	logrus.Debugf("entering SetupElegantExit")
	// handle elegant exit; create support for channels of go routines that want to exit cleanly
	registeredChannels = make([]chan bool, 0)
	exitSigChan := make(chan os.Signal, 1)
	logrus.Debugf("registered exit signal channel")
	signal.Notify(exitSigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		// wait for exit signal; then stop all the other go functions
		sig := <-exitSigChan
		logrus.Debugf("received exit signal = %v", sig)
		// exit signal received; stop other go functions
		for _, ch := range registeredChannels {
			ch <- true
		}
		logrus.Debugf("exiting SetupElegantExit go function")
	}()
	logrus.Debugf("exiting SetupElegantExit")
}
