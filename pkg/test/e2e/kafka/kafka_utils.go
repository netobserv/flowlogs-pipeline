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
	"time"

	kafkago "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

const (
	defaultInputFile        = "../../../../hack/examples/ocp-ipfix-flowlogs.json"
	inputFileEnvVar         = "INPUT_FILE"
	kafkaInputTopicDefault  = "test_topic_in"
	kafkaOutputTopicDefault = "test_topic_out"
)

var kafkaInputTopic string
var kafkaOutputTopic string
var theKafkaServer string

type lineBuffer []byte

func createKafkaProducer(t *testing.T) *kafkago.Writer {
	// prepare a kafka producer on specified topic
	topic := kafkaInputTopic
	kafkaClient := kafkago.Client{
		Addr: kafkago.TCP(theKafkaServer),
	}

	fmt.Printf("kafka Server: %s \n", theKafkaServer)
	fmt.Printf("creating producer topic: %s \n", topic)
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
		Addr:  kafkago.TCP(theKafkaServer),
		Topic: topic,
	}
	return &kafkaProducer
}

func createKafkaConsumer(t *testing.T) *kafkago.Reader {
	topic := kafkaOutputTopic
	kafkaClient := kafkago.Client{
		Addr: kafkago.TCP(theKafkaServer),
	}

	fmt.Printf("kafka Server: %s \n", theKafkaServer)
	fmt.Printf("creating consumer topic: %s \n", topic)
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
		Brokers:     []string{theKafkaServer},
		Topic:       topic,
		StartOffset: kafkago.LastOffset,
	})
	return kafkaConsumer
}

func sendKafkaData(t *testing.T, producer *kafkago.Writer, inputData []lineBuffer) {
	var msgs []kafkago.Message
	msgs = make([]kafkago.Message, 0)
	for _, entry := range inputData {
		msg := kafkago.Message{
			Value: entry,
		}
		msgs = append(msgs, msg)
	}

	err := producer.WriteMessages(context.Background(), msgs...)
	if err != nil {
		msg := fmt.Sprintf("error conecting to kafka; cannot perform kafka end-to-end test; err = %v \n", err)
		assert.Fail(t, msg)
	}
	assert.NoError(t, err)
}

func getInput(t *testing.T) []lineBuffer {
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
	return lines
}

func receiveData(consumer *kafkago.Reader, nLines int) []lineBuffer {
	fmt.Printf("receiveData:  nLines = %d \n", nLines)
	output := make([]lineBuffer, nLines)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for i := 0; i < nLines; i++ {
		kafkaMessage, err := consumer.ReadMessage(ctx)
		if err != nil {
			panic("Kafka: receiveData: ReadMessage timeout (after 10 minutes)")
		}
		output[i] = kafkaMessage.Value
		fmt.Printf(".")
	}
	fmt.Printf("\n")
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
