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

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/vladimirvivien/gexe"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func postFLP1Deploy(_ context.Context, cfg *envconf.Config, namespace string) error {
	fmt.Printf("====> Waiting for FLP1 Pod (emitter)\n")
	client, err := cfg.NewClient()
	if err != nil {
		return fmt.Errorf("postFLP1Deploy:NewClient error: %w", err)
	}
	j := batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "flp-1", Namespace: namespace}}
	err = wait.For(
		conditions.New(client.Resources()).JobCompleted(&j), wait.WithTimeout(time.Minute*3),
	)
	if err != nil {
		return fmt.Errorf("postFLP1Deploy:wait.For error: %w", err)
	}
	fmt.Printf("====> Done.\n")
	return nil
}

func postFLP2Deploy(_ context.Context, cfg *envconf.Config, namespace string) error {
	fmt.Printf("====> Waiting for FLP2 Pod (receiver)\n")
	client, err := cfg.NewClient()
	if err != nil {
		return fmt.Errorf("postFLP2Deploy:NewClient error: %w", err)
	}
	p := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "flp-2", Namespace: namespace}}
	err = wait.For(
		conditions.New(client.Resources()).PodRunning(&p), wait.WithTimeout(time.Minute*3),
	)
	if err != nil {
		return fmt.Errorf("postFLP2Deploy:wait.For error: %w", err)
	}
	fmt.Printf("====> Done.\n")
	return nil
}

func postStrimziDeploy(_ context.Context, cfg *envconf.Config, namespace string) error {
	fmt.Printf("====> Waiting for Strimzi and Kafka Deployment\n")
	client, err := cfg.NewClient()
	if err != nil {
		return fmt.Errorf("postStrimziDeploy:NewClient error: %w", err)
	}
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "strimzi-cluster-operator", Namespace: namespace},
	}
	// wait for the deployment to finish becoming available
	err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(time.Minute*10))
	if err != nil {
		return fmt.Errorf("postStrimziDeploy:wait.For error: %w", err)
	}

	// wait for kafka to be ready
	res := gexe.RunProc(fmt.Sprintf("kubectl wait kafka/my-cluster --for=condition=Ready --timeout=20m -n %s", namespace))
	if !res.IsSuccess() {
		return fmt.Errorf("postStrimziKafkaDeploy error: %w", res.Err())
	}
	fmt.Printf("====> Done.\n")
	return nil
}

func TestKafka_Basic(t *testing.T) {
	pipelineFeature := features.New("FLP/kafka").WithLabel("env", "dev").
		Setup(func(ctx context.Context, _ *testing.T, _ *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Kafka working as expected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			p := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "flp-2", Namespace: cfg.Namespace()}}

			// coreV1Client from current context in kubeconfig
			coreV1Client, err := e2e.GetCoreV1Client(cfg.KubeconfigFile())
			if err != nil {
				t.Fatal(err)
			}

			logs := e2e.LogsFromPods([]corev1.Pod{p}, coreV1Client, cfg.Namespace())
			fmt.Print(logs)

			assert.Contains(t, logs, "Starting flowlogs-pipeline", "can't find flowlogs-pipeline start message")
			assert.NotContains(t, logs, "ERROR", "flowlogs-pipeline reports errors")

			assert.Contains(t, logs, "Bytes:116000", "Missing input data")
			assert.Contains(t, logs, "ProcessedByFLP1:true", "Missing processing confirmation from FLP 1")
			assert.Contains(t, logs, "ProcessedByFLP2:true", "Missing processing confirmation from FLP 2")
			return ctx
		}).Feature()

	TestEnv.Test(t, pipelineFeature)
}
