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

package informers

import (
	"context"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/metrics"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

const (
	leaseName      = "flp-informers-lease"
	leaseDuration  = 15 * time.Second
	renewDeadline  = 10 * time.Second
	retryPeriod    = 2 * time.Second
)

// LeaderElectionConfig holds configuration for leader election
type LeaderElectionConfig struct {
	Enabled   bool
	Namespace string
	Identity  string
}

// RunWithLeaderElection runs the main logic with leader election
// Only the elected leader will execute runFunc, others will standby
func RunWithLeaderElection(ctx context.Context, config LeaderElectionConfig, healthServer *HealthServer, runFunc func(context.Context)) error {
	if !config.Enabled {
		log.Info("Leader election disabled - running as single instance")
		healthServer.SetLeader(true)
		healthServer.SetReady(true)
		metrics.InformersMetrics.IsLeader.Set(1)
		runFunc(ctx)
		return nil
	}

	// Get in-cluster config
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return err
	}

	// Create resource lock for leader election
	// Using Lease as it's the recommended lock type
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseName,
			Namespace: config.Namespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: config.Identity,
		},
	}

	// Mark as ready once we can participate in leader election
	// Both leader and followers are considered "ready" for K8s purposes
	healthServer.SetReady(true)

	// Start leader election
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   leaseDuration,
		RenewDeadline:   renewDeadline,
		RetryPeriod:     retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Info("Started leading - running informers")
				healthServer.SetLeader(true)
				metrics.InformersMetrics.IsLeader.Set(1)
				runFunc(ctx)
			},
			OnStoppedLeading: func() {
				log.Info("Stopped leading")
				healthServer.SetLeader(false)
				metrics.InformersMetrics.IsLeader.Set(0)
			},
			OnNewLeader: func(identity string) {
				if identity == config.Identity {
					log.Info("I am the new leader")
				} else {
					log.WithField("leader", identity).Info("New leader elected")
				}
			},
		},
	})

	return nil
}
