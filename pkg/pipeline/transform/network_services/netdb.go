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
 * > Note: this code is a revised and enhanced version of the netdb.go file
 * > from https://github.com/dominikh/go-netdb/ (MIT License)
 */

package netdb

import (
	"io/ioutil"
	"strconv"
	"strings"
)

type Protoent struct {
	Name    string
	Aliases []string
	Number  int
}

type Servent struct {
	Name     string
	Aliases  []string
	Port     int
	Protocol *Protoent
}

// These variables get populated from /etc/protocols and /etc/services
// respectively.
var (
	Protocols []*Protoent
	Services  []*Servent
)

func Init(protocolsFile, servicesFile string) error {
	Protocols = []*Protoent{}
	Services = []*Servent{}

	protoMap := make(map[string]*Protoent)

	// Load protocols
	if protocolsFile == "" {
		protocolsFile = "/etc/protocols"
	}
	data, err := ioutil.ReadFile(protocolsFile)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		split := strings.SplitN(line, "#", 2)
		fields := strings.Fields(split[0])
		if len(fields) < 2 {
			continue
		}

		num, err := strconv.ParseInt(fields[1], 10, 32)
		if err != nil {
			return err
		}

		protoent := &Protoent{
			Name:    fields[0],
			Aliases: fields[2:],
			Number:  int(num),
		}
		Protocols = append(Protocols, protoent)

		protoMap[fields[0]] = protoent
	}

	// Load services
	if servicesFile == "" {
		servicesFile = "/etc/services"
	}
	data, err = ioutil.ReadFile(servicesFile)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		split := strings.SplitN(line, "#", 2)
		fields := strings.Fields(split[0])
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		portproto := strings.SplitN(fields[1], "/", 2)
		port, err := strconv.ParseInt(portproto[0], 10, 32)
		if err != nil {
			return err
		}

		proto := portproto[1]
		aliases := fields[2:]

		Services = append(Services, &Servent{
			Name:     name,
			Aliases:  aliases,
			Port:     int(port),
			Protocol: protoMap[proto],
		})
	}

	return nil
}

// Equal checks if two Protoents are the same, which is the case if
// their protocol numbers are identical or when both Protoents are
// nil.
func (this *Protoent) Equal(other *Protoent) bool {
	if this == nil && other == nil {
		return true
	}

	if this == nil || other == nil {
		return false
	}

	return this.Number == other.Number
}

// Equal checks if two Servents are the same, which is the case if
// their port numbers and protocols are identical or when both
// Servents are nil.
func (this *Servent) Equal(other *Servent) bool {
	if this == nil && other == nil {
		return true
	}

	if this == nil || other == nil {
		return false
	}

	return this.Port == other.Port &&
		this.Protocol.Equal(other.Protocol)
}

// GetProtoByNumber returns the Protoent for a given protocol number.
func GetProtoByNumber(num int) (protoent *Protoent) {
	for _, protoent := range Protocols {
		if protoent.Number == num {
			return protoent
		}
	}
	return nil
}

// GetProtoByName returns the Protoent whose name or any of its
// aliases matches the argument.
func GetProtoByName(name string) (protoent *Protoent) {
	for _, protoent := range Protocols {
		if protoent.Name == name {
			return protoent
		}

		for _, alias := range protoent.Aliases {
			if alias == name {
				return protoent
			}
		}
	}

	return nil
}

// GetServByName returns the Servent for a given service name or alias
// and protocol. If the protocol is nil, the first service matching
// the service name is returned.
func GetServByName(name string, protocol *Protoent) (servent *Servent) {
	for _, servent := range Services {
		if !servent.Protocol.Equal(protocol) {
			continue
		}

		if servent.Name == name {
			return servent
		}

		for _, alias := range servent.Aliases {
			if alias == name {
				return servent
			}
		}
	}

	return nil
}

// GetServByPort returns the Servent for a given port number and
// protocol. If the protocol is nil, the first service matching the
// port number is returned.
func GetServByPort(port int, protocol *Protoent) *Servent {
	for _, servent := range Services {
		if servent.Port == port && servent.Protocol.Equal(protocol) {
			return servent
		}
	}

	return nil
}
