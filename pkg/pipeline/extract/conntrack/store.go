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

package conntrack

import (
	"container/list"
	"time"

	log "github.com/sirupsen/logrus"
)

// connectionStore provides both retrieving a connection by its hash and iterating connections sorted by their last
// update time.
type connectionStore struct {
	hash2conn map[uint64]*list.Element
	connList  *list.List
}

type processConnF func(connection) (shouldDelete, shouldStop bool)

func (cs *connectionStore) addConnection(hashId uint64, conn connection) {
	_, ok := cs.getConnection(hashId)
	if ok {
		log.Errorf("BUG. connection with hash %x already exists in store. %v", hashId, conn)
	}
	e := cs.connList.PushBack(conn)
	cs.hash2conn[hashId] = e
	metrics.connStoreLength.Set(float64(cs.connList.Len()))
}

func (cs *connectionStore) getConnection(hashId uint64) (connection, bool) {
	elem, ok := cs.hash2conn[hashId]
	if ok {
		conn := elem.Value.(connection)
		return conn, ok
	}
	return nil, ok
}

func (cs *connectionStore) updateConnectionTime(hashId uint64, t time.Time) {
	elem, ok := cs.hash2conn[hashId]
	if !ok {
		log.Errorf("BUG. connection hash %x doesn't exist", hashId)
		return
	}
	elem.Value.(connection).setLastUpdate(t)
	// move to end of list
	cs.connList.MoveToBack(elem)
}

func (cs *connectionStore) iterateOldToNew(f processConnF) {
	// How to remove element from list while iterating the same list in golang
	// https://stackoverflow.com/a/27662823/2749989
	var next *list.Element
	for e := cs.connList.Front(); e != nil; e = next {
		conn := e.Value.(connection)
		next = e.Next()
		shouldDelete, shouldStop := f(conn)
		if shouldDelete {
			delete(cs.hash2conn, conn.getHash().hashTotal)
			cs.connList.Remove(e)
			metrics.connStoreLength.Set(float64(cs.connList.Len()))
		}
		if shouldStop {
			break
		}
	}
}

func newConnectionStore() *connectionStore {
	return &connectionStore{
		hash2conn: make(map[uint64]*list.Element),
		connList:  list.New(),
	}
}
