/*
 * Copyright (C) 2024 Red Hat, Inc.
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

package k8scache

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/metrics"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

// clientSender defines the interface for sending cache updates.
// This interface allows for easier testing by enabling mock implementations.
type clientSender interface {
	SendAdd(entries []*model.ResourceMetaData) error
	SendUpdate(entries []*model.ResourceMetaData) error
	SendDelete(entries []*model.ResourceMetaData) error
}

// EventHandler implements informers.EventHandler to push cache updates via gRPC.
// It handles Kubernetes resource events (Add, Update, Delete) and forwards them
// to connected FLP processor pods through the k8scache client.
type EventHandler struct {
	client clientSender
}

// NewEventHandler creates a new event handler that forwards K8s events to the given client
func NewEventHandler(client *Client) *EventHandler {
	return &EventHandler{client: client}
}

// OnAdd is called when a new resource is added to the informer cache.
// It skips resources from the initial list (isInInitialList=true) to avoid
// sending full snapshots, and only forwards incremental additions.
func (h *EventHandler) OnAdd(obj interface{}, isInInitialList bool) {
	if isInInitialList {
		// Skip initial list - we send incremental updates only
		return
	}

	meta, ok := obj.(*model.ResourceMetaData)
	if !ok {
		// Kubernetes sometimes sends partial metadata objects for optimization.
		// These don't have the full info we need (IPs, etc), so we skip them.
		log.Debugf("skipping partial metadata object in OnAdd: %T", obj)
		return
	}

	if err := h.client.SendAdd([]*model.ResourceMetaData{meta}); err != nil {
		log.WithError(err).WithField("resource", meta.Name).Error("failed to send ADD")
	} else {
		if metrics.InformersMetrics != nil {
			metrics.InformersMetrics.CacheUpdatesTotal.WithLabelValues("ADD").Inc()
		}
	}
}

// OnUpdate is called when a resource is updated in the informer cache.
// It forwards the new state to all connected processors.
func (h *EventHandler) OnUpdate(_, newObj interface{}) {
	meta, ok := newObj.(*model.ResourceMetaData)
	if !ok {
		// Kubernetes sometimes sends partial metadata objects for optimization.
		// These don't have the full info we need (IPs, etc), so we skip them.
		log.Debugf("skipping partial metadata object in OnUpdate: %T", newObj)
		return
	}

	if err := h.client.SendUpdate([]*model.ResourceMetaData{meta}); err != nil {
		log.WithError(err).WithField("resource", meta.Name).Error("failed to send UPDATE")
	} else {
		if metrics.InformersMetrics != nil {
			metrics.InformersMetrics.CacheUpdatesTotal.WithLabelValues("UPDATE").Inc()
		}
	}
}

// OnDelete is called when a resource is deleted from the informer cache.
// It handles both normal delete events and tombstones (DeletedFinalStateUnknown).
//
// Tombstones occur when the informer misses a delete event (e.g., due to temporary
// disconnection). In this case, Kubernetes sends a DeletedFinalStateUnknown object
// containing the last known state of the deleted resource. Without proper handling,
// these missed deletes would leave stale entries in the cache.
func (h *EventHandler) OnDelete(obj interface{}) {
	var meta *model.ResourceMetaData
	var ok bool

	// Handle tombstones: when an informer misses a delete event, it can send a
	// DeletedFinalStateUnknown object containing the last known state
	if tombstone, isTombstone := obj.(cache.DeletedFinalStateUnknown); isTombstone {
		// Extract the actual object from the tombstone
		meta, ok = tombstone.Obj.(*model.ResourceMetaData)
		if !ok {
			// Kubernetes sometimes sends partial metadata objects for optimization.
			log.Debugf("tombstone contained partial metadata object in OnDelete: %T", tombstone.Obj)
			return
		}
		log.Debugf("recovered delete event from tombstone for resource: %s", meta.Name)
	} else {
		// Not a tombstone, try direct conversion
		meta, ok = obj.(*model.ResourceMetaData)
		if !ok {
			// Kubernetes sometimes sends partial metadata objects for optimization.
			log.Debugf("skipping partial metadata object in OnDelete: %T", obj)
			return
		}
	}

	if err := h.client.SendDelete([]*model.ResourceMetaData{meta}); err != nil {
		log.WithError(err).WithField("resource", meta.Name).Error("failed to send DELETE")
	} else {
		if metrics.InformersMetrics != nil {
			metrics.InformersMetrics.CacheUpdatesTotal.WithLabelValues("DELETE").Inc()
		}
	}
}
