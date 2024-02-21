/*
 * Copyright (C) 2021 IBM, Inc.
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

package write

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/sirupsen/logrus"
)

type writeTCP struct {
	address string
	conn    net.Conn
}

// Write writes a flow to tcp connection
func (t *writeTCP) Write(v config.GenericMap) {
	logrus.Tracef("entering writeTCP Write")
	b, _ := json.Marshal(v)
	// append new line between each record to split on client side
	b = append(b, []byte("\n")...)
	_, err := t.conn.Write(b)
	if err != nil {
		log.WithError(err).Warn("can't write tcp")
	}
}

// NewWriteTCP create a new write
func NewWriteTCP(params config.StageParam) (Writer, error) {
	logrus.Debugf("entering NewWriteTCP")
	writeTCP := &writeTCP{}
	if params.Write != nil && params.Write.TCP != nil && params.Write.TCP.Port != "" {
		writeTCP.address = ":" + params.Write.TCP.Port
	} else {
		return nil, fmt.Errorf("Write.TCP.Port must be specified")
	}

	logrus.Debugf("NewWriteTCP Listen %s", writeTCP.address)
	l, err := net.Listen("tcp", writeTCP.address)
	if err != nil {
		return nil, err
	}
	defer l.Close()
	clientConn, err := l.Accept()
	if err != nil {
		return nil, err
	}
	writeTCP.conn = clientConn

	return writeTCP, nil
}
