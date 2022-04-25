/*
 * Copyright (C) 2022 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package utils

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
)

var (
	registeredChannels []chan struct{}
	chanMutex          sync.Mutex
)

func RegisterExitChannel(ch chan struct{}) {
	chanMutex.Lock()
	defer chanMutex.Unlock()
	registeredChannels = append(registeredChannels, ch)
}

func SetupElegantExit() {
	log.Debugf("entering SetupElegantExit")
	// handle elegant exit; create support for channels of go routines that want to exit cleanly
	registeredChannels = make([]chan struct{}, 0)
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
			close(ch)
		}
		log.Debugf("exiting SetupElegantExit go function")
	}()
	log.Debugf("exiting SetupElegantExit")
}
