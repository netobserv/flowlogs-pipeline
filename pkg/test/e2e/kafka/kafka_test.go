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
	"strings"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/netobserv/flowlogs-pipeline/pkg/test/e2e"
	"github.com/vladimirvivien/gexe"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func postFLPDeploy(_ context.Context, cfg *envconf.Config, namespace string) error {
	fmt.Printf("====> Waiting for FLP Deployment\n")
	client, err := cfg.NewClient()
	if err != nil {
		return fmt.Errorf("postFLPDeploy:NewClient error: %w", err)
	}
	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "flowlogs-pipeline", Namespace: namespace},
	}
	// wait for the deployment to finish becoming available
	err = wait.For(
		conditions.New(client.Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(time.Minute*3))
	if err != nil {
		return fmt.Errorf("postFLPDeploy:wait.For error: %w", err)
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
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			dep := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "flowlogs-pipeline", Namespace: cfg.Namespace()},
			}
			// wait for the deployment to finish becoming available
			err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(time.Minute*3))
			if err != nil {
				t.Fatal(err)
			}

			// coreV1Client from current context in kubeconfig
			coreV1Client, err := e2e.GetCoreV1Client(cfg.KubeconfigFile())
			if err != nil {
				t.Fatal(err)
			}

			var pods corev1.PodList
			err = client.Resources(cfg.Namespace()).List(context.TODO(), &pods, resources.WithLabelSelector("app=flowlogs-pipeline"))
			if err != nil {
				t.Fatal(err)
			}

			logs := e2e.LogsFromPods(&pods, coreV1Client, cfg.Namespace())
			fmt.Print(logs)
			startExist := strings.Contains(logs, "Starting flowlogs-pipeline")
			if !startExist {
				t.Fatal("can't find flowlogs-pipeline start message")
			}
			errorsExist := strings.Contains(logs, "ERROR")
			if errorsExist {
				t.Fatal("flowlogs-pipeline reports errors ")
			}

			command := "kubectl get kafka my-cluster -o=jsonpath='{.status.listeners[?(@.type==\"external\")].bootstrapServers}'"
			kafkaAddr := test.RunCommand(command)
			// strip the quotation marks
			theKafkaServer = strings.Trim(kafkaAddr, "'`{}[]\"")
			fmt.Printf("KafkaServer = %s \n", theKafkaServer)

			kafkaInputTopic = kafkaInputTopicDefault
			kafkaOutputTopic = kafkaOutputTopicDefault

			fmt.Printf("create kafka producer \n")
			producer := createKafkaProducer(t)
			fmt.Printf("create kafka consumer \n")
			consumer := createKafkaConsumer(t)
			fmt.Printf("read input \n")
			input := getInput(t)
			nLines := len(input)
			fmt.Printf("send input data to kafka input topic \n")
			sendKafkaData(t, producer, input)
			fmt.Printf("read data from kafka output topic \n")
			output := receiveData(consumer, nLines)
			fmt.Printf("check results \n")
			checkResults(t, input, output)

			return ctx
		}).Feature()

	TestEnv.Test(t, pipelineFeature)
}
