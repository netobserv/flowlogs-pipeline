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
	"bufio"
	"fmt"
	"os"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	kafkago "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

const (
	defaultInputFile = "../../../../hack/examples/ocp-ipfix-flowlogs.json"
	inputFileEnvVar  = "INPUT_FILE"
	kafkaInputTopic  = "test_topic_in"
	kafkaOutputTopic = "test_topic_out"
)

var kafkaExternalAddr string

type lineBuffer []byte

func makeKafkaProducer(t *testing.T) *kafkago.Writer {
	// prepare a kafka producer on specified topic
	topic := kafkaInputTopic
	fmt.Printf("producer topic = %s \n", topic)
	fmt.Printf("kafkaServer = %s \n", kafkaExternalAddr)
	kafkaClient := kafkago.Client{
		Addr: kafkago.TCP(kafkaExternalAddr),
	}

	/*
		deleteResponse, err := kafkaClient.DeleteTopics(context.Background(), &kafkago.DeleteTopicsRequest{
			Topics: []string{topic},
		})
		log.Debugf("DeleteTopics response = %v, err = %v \n", deleteResponse, err)
		fmt.Printf("wait a second for topic deletion to complete \n")
		time.Sleep(time.Second)
	*/

	fmt.Printf("create topic: %s \n", topic)
	createResponse, err := kafkaClient.CreateTopics(context.Background(), &kafkago.CreateTopicsRequest{
		Topics: []kafkago.TopicConfig{
			{Topic: topic,
				NumPartitions:     1,
				ReplicationFactor: 1}},
	})
	assert.NoError(t, err)
	log.Debugf("CreateTopics response = %v, err = %v \n", createResponse, err)

	electResponse, err := kafkaClient.ElectLeaders(context.Background(), &kafkago.ElectLeadersRequest{
		Topic: topic,
	})
	assert.NoError(t, err)
	log.Debugf("ElectLeaders response = %v, err = %v \n", electResponse, err)
	kafkaProducer := kafkago.Writer{
		Addr:  kafkago.TCP(kafkaExternalAddr),
		Topic: topic,
	}
	return &kafkaProducer
}

func makeKafkaConsumer(t *testing.T) *kafkago.Reader {
	topic := kafkaOutputTopic
	fmt.Printf("consumer topic = %s \n", topic)

	kafkaClient := kafkago.Client{
		Addr: kafkago.TCP(kafkaExternalAddr),
	}
	/*
		deleteResponse, err := kafkaClient.DeleteTopics(context.Background(), &kafkago.DeleteTopicsRequest{
			Topics: []string{topic},
		})
		assert.NoError(t, err)
		log.Debugf("DeleteTopics response = %v, err = %v \n", deleteResponse, err)
		fmt.Printf("wait a second for topic deletion to complete \n")
		time.Sleep(time.Second)
	*/
	fmt.Printf("create topic: %s \n", topic)
	createTopicsResponse, err := kafkaClient.CreateTopics(context.Background(), &kafkago.CreateTopicsRequest{
		Topics: []kafkago.TopicConfig{
			{Topic: topic,
				NumPartitions:     1,
				ReplicationFactor: 1}},
	})
	assert.NoError(t, err)
	log.Debugf("createTopicsResponse response = %v, err = %v \n", createTopicsResponse, err)

	electResponse, err := kafkaClient.ElectLeaders(context.Background(), &kafkago.ElectLeadersRequest{
		Topic: topic,
	})
	assert.NoError(t, err)
	log.Debugf("ElectLeaders response = %v, err = %v \n", electResponse, err)

	// prepare a kafka consumer on specified topic
	kafkaConsumer := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     []string{kafkaExternalAddr},
		Topic:       topic,
		StartOffset: kafkago.LastOffset,
	})
	return kafkaConsumer
}

func sendKafkaData(t *testing.T, producer *kafkago.Writer, inputData []lineBuffer) {
	fmt.Printf("entering sendKafkaData \n")
	var msgs []kafkago.Message
	msgs = make([]kafkago.Message, 0)
	k := 0
	for _, entry := range inputData {
		msg := kafkago.Message{
			Value: entry,
		}
		msgs = append(msgs, msg)
		k++
		if k > 100 {
			fmt.Printf("\n")
			k = 0
		}
	}

	err := producer.WriteMessages(context.Background(), msgs...)
	if err != nil {
		msg := fmt.Sprintf("error conecting to kafka; cannot perform kafka end-to-end test; err = %v \n", err)
		assert.Fail(t, msg)
	}
	assert.NoError(t, err)
	fmt.Printf("exiting sendKafkaData \n")
}

func getInput(t *testing.T) []lineBuffer {
	fmt.Printf("entering getInput \n")
	inputFile := os.Getenv(inputFileEnvVar)
	if inputFile == "" {
		inputFile = defaultInputFile
	}
	fmt.Printf("input file = %v \n", inputFile)
	file, err := os.Open(inputFile)
	assert.NoError(t, err)
	defer func() {
		_ = file.Close()
	}()

	lines := make([]lineBuffer, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Bytes()
		text2 := make(lineBuffer, len(text))
		copy(text2, text)
		lines = append(lines, text2)
	}
	fmt.Printf("exiting getInput \n")
	return lines
}

func receiveData(t *testing.T, consumer *kafkago.Reader, nLines int) []lineBuffer {
	fmt.Printf("entering receiveData \n")
	fmt.Printf("receiveData:  nLines = %d \n", nLines)
	output := make([]lineBuffer, nLines)
	k := 0
	for i := 0; i < nLines; i++ {
		kafkaMessage, _ := consumer.ReadMessage(context.Background())
		output[i] = kafkaMessage.Value
		fmt.Printf(".")
		k++
		if k > 100 {
			fmt.Printf("\n")
			k = 0
		}
	}
	fmt.Printf("\n")
	fmt.Printf("exiting receiveData \n")
	return output
}

func checkResults(t *testing.T, input, output []lineBuffer) {
	assert.Equal(t, len(input), len(output))
	for _, line := range input {
		fmt.Printf(".")
		assert.Contains(t, output, line)
	}
	fmt.Printf("\n")
}

func TestEnd2EndKafka(t *testing.T) {
	command := "kubectl get kafka my-cluster -o=jsonpath='{.status.listeners[?(@.type==\"external\")].bootstrapServers}'"
	kafkaAddr, err := test.RunCommand(command)
	assert.NoError(t, err)
	// strip the quotation marks
	kafkaExternalAddr = kafkaAddr[1 : len(kafkaAddr)-1]
	fmt.Printf("kafkaExternalAddr = %s \n", kafkaExternalAddr)

	fmt.Printf("create kafka producer \n")
	producer := makeKafkaProducer(t)
	fmt.Printf("create kafka consumer \n")
	consumer := makeKafkaConsumer(t)
	fmt.Printf("set up pipeline \n")
	// pipeline is not run inside the cluster
	//setupPipeline(t)
	fmt.Printf("read input \n")
	input := getInput(t)
	nLines := len(input)
	fmt.Printf("send input data to kafka input topic \n")
	sendKafkaData(t, producer, input)
	fmt.Printf("read data from kafka output topic \n")
	output := receiveData(t, consumer, nLines)
	fmt.Printf("check results \n")
	checkResults(t, input, output)
}
