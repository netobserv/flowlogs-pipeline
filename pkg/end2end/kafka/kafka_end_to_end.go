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

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline"
	kafkago "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

const (
	defaultInputFile        = "goflow2_input.txt"
	kafkaBrokerDefaultAddr  = "localhost:9092"
	inputFileEnvVar         = "INPUT_FILE"
	kafkaBrokerEnvVar       = "KAFKA_BROKER"
	kafkaInputTopicEnvVar   = "KAFKA_INPUT_TOPIC"
	kafkaOutputTopicEnvVar  = "KAFKA_OUTPUT_TOPIC"
	kafkaInputTopicDefault  = "topic_in"
	kafkaOutputTopicDefault = "topic_out"
	dockerComposeFile       = "docker-compose.yaml"
)

type lineBuffer []byte

func makeKafkaProducer() kafkago.Writer {
	// prepare a kafka producer on specified topic
	kafkaAddr := os.Getenv(kafkaBrokerEnvVar)
	if kafkaAddr == "" {
		kafkaAddr = kafkaBrokerDefaultAddr
	}
	fmt.Printf("kafkaAddr = %s \n", kafkaAddr)
	topic := os.Getenv(kafkaInputTopicEnvVar)
	if topic == "" {
		topic = kafkaInputTopicDefault
	}
	fmt.Printf("producer topic = %s \n", topic)
	kafkaClient := kafkago.Client{
		Addr: kafkago.TCP(kafkaAddr),
	}

	deleteResponse, err := kafkaClient.DeleteTopics(context.Background(), &kafkago.DeleteTopicsRequest{
		Topics: []string{topic},
	})
	fmt.Printf("DeleteTopics response = %v, err = %v \n", deleteResponse, err)
	fmt.Printf("wait a second for topic deletion to complete \n")
	time.Sleep(time.Second)

	createResponse, err := kafkaClient.CreateTopics(context.Background(), &kafkago.CreateTopicsRequest{
		Topics: []kafkago.TopicConfig{
			{Topic: topic,
				NumPartitions:     1,
				ReplicationFactor: 1}},
	})
	fmt.Printf("CreateTopics response = %v, err = %v \n", createResponse, err)

	electResponse, err := kafkaClient.ElectLeaders(context.Background(), &kafkago.ElectLeadersRequest{
		Topic: topic,
	})
	fmt.Printf("ElectLeaders response = %v, err = %v \n", electResponse, err)
	kafkaProducer := kafkago.Writer{
		Addr:  kafkago.TCP(kafkaAddr),
		Topic: topic,
	}
	return kafkaProducer
}

func makeKafkaConsumer() kafkago.Reader {
	kafkaAddr := os.Getenv(kafkaBrokerEnvVar)
	if kafkaAddr == "" {
		kafkaAddr = kafkaBrokerDefaultAddr
	}
	fmt.Printf("kafkaAddr = %s \n", kafkaAddr)
	topic := os.Getenv(kafkaOutputTopicEnvVar)
	if topic == "" {
		topic = kafkaOutputTopicDefault
	}
	fmt.Printf("consumer topic = %s \n", topic)

	kafkaClient := kafkago.Client{
		Addr: kafkago.TCP(kafkaAddr),
	}

	deleteResponse, err := kafkaClient.DeleteTopics(context.Background(), &kafkago.DeleteTopicsRequest{
		Topics: []string{topic},
	})
	fmt.Printf("DeleteTopics response = %v, err = %v \n", deleteResponse, err)
	fmt.Printf("wait a second for topic deletion to complete \n")
	time.Sleep(time.Second)

	kafkaClient.CreateTopics(context.Background(), &kafkago.CreateTopicsRequest{
		Topics: []kafkago.TopicConfig{
			{Topic: topic,
				NumPartitions:     1,
				ReplicationFactor: 1}},
	})

	electResponse, err := kafkaClient.ElectLeaders(context.Background(), &kafkago.ElectLeadersRequest{
		Topic: topic,
	})
	fmt.Printf("ElectLeaders response = %v, err = %v \n", electResponse, err)

	// prepare a kafka consumer on specified topic
	kafkaConsumer := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     []string{kafkaAddr},
		Topic:       topic,
		StartOffset: kafkago.LastOffset,
	})
	return *kafkaConsumer
}

func sendKafkaData(producer kafkago.Writer, inputData []lineBuffer) {
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
		fmt.Printf("error conecting to kafka; cannot perform kafka end-to-end test; err = %v \n", err)
		os.Exit(1)
	}
}

func getInput() []lineBuffer {
	inputFile := os.Getenv(inputFileEnvVar)
	if inputFile == "" {
		inputFile = defaultInputFile
	}
	fmt.Printf("input file = %v \n", inputFile)
	file, err := os.Open(inputFile)
	if err != nil {
		log.Fatal(err)
	}
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

func receiveData(consumer kafkago.Reader, nLines int) []lineBuffer {
	fmt.Printf("inside receiveData \n")
	fmt.Printf(" nLines = %d \n", nLines)
	output := make([]lineBuffer, nLines)
	for i := 0; i < nLines; i++ {
		kafkaMessage, _ := consumer.ReadMessage(context.Background())
		output[i] = kafkaMessage.Value
	}
	fmt.Printf("exiting receiveData \n")
	return output
}

func checkResults(input, output []lineBuffer) {
	for i, line := range input {
		if !bytes.Equal(output[i], line) {
			fmt.Printf("output does not equal input, i = %d \n input = %s \n output = %s \n", i, string(line), string(output[i]))
			os.Exit(1)
		}
	}
	fmt.Printf("SUCCESS \n")
}

func setupPipeline() {

	kafkaAddr := os.Getenv(kafkaBrokerEnvVar)
	if kafkaAddr == "" {
		kafkaAddr = kafkaBrokerDefaultAddr
	}
	fmt.Printf("kafkaAddr = %s \n", kafkaAddr)

	kafkaConfigTemplate := fmt.Sprintf(`
pipeline:
  - name: kafka_ingest
  - name: decode_json
    follows: kafka_ingest
  - name: transform_none
    follows: decode_json
  - name: kafka_encode
    follows: transform_none
  - name: write_none
    follows: kafka_encode
parameters:
  - name: kafka_ingest
    ingest:
      type: kafka
      kafka:
        brokers: [%s]
        topic: topic_in
        groupid: group_test_in
  - name: decode_json
    decode:
      type: json
  - name: transform_none
    transform:
      type: none
  - name: kafka_encode
    encode:
      type: kafka
      kafka:
        address: %s
        topic: topic_out
  - name: write_none
    write:
      type: none
`, kafkaAddr, kafkaAddr)

	var err error
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	yamlConfig := []byte(kafkaConfigTemplate)
	v := viper.New()
	v.SetConfigType("yaml")
	r := bytes.NewReader(yamlConfig)
	err = v.ReadConfig(r)
	if err != nil {
		fmt.Printf("Error reading config file, err = %v \n", err)
		os.Exit(1)
	}

	var b []byte
	pipelineStr := v.Get("pipeline")
	b, err = json.Marshal(&pipelineStr)
	if err != nil {
		fmt.Printf("error marshaling: %v\n", err)
		os.Exit(1)
	}
	config.Opt.PipeLine = string(b)
	parametersStr := v.Get("parameters")
	b, err = json.Marshal(&parametersStr)
	if err != nil {
		fmt.Printf("error marshaling: %v\n", err)
		os.Exit(1)
	}
	config.Opt.Parameters = string(b)
	err = json.Unmarshal([]byte(config.Opt.PipeLine), &config.PipeLine)
	if err != nil {
		fmt.Printf("error unmarshaling: %v\n", err)
		os.Exit(1)
	}
	err = json.Unmarshal([]byte(config.Opt.Parameters), &config.Parameters)
	if err != nil {
		fmt.Printf("error unmarshaling: %v\n", err)
		os.Exit(1)
	}

	err = config.ParseConfig()
	if err != nil {
		fmt.Printf("error in parsing config file: %v \n", err)
		os.Exit(1)
	}

	var mainPipeline *pipeline.Pipeline
	mainPipeline, _ = pipeline.NewPipeline()

	// run the pipeline in a separate go-routine
	go func() {
		mainPipeline.Run()
	}()
}

func main() {
	fmt.Printf("before makeKafkaProducer \n")
	producer := makeKafkaProducer()
	fmt.Printf("before makeKafkaConsumer \n")
	consumer := makeKafkaConsumer()
	fmt.Printf("before setupPipeline \n")
	setupPipeline()
	fmt.Printf("before getInput \n")
	input := getInput()
	nLines := len(input)
	fmt.Printf("before sendKafkaData \n")
	sendKafkaData(producer, input)
	fmt.Printf("before receiveData \n")
	output := receiveData(consumer, nLines)
	fmt.Printf("before checkResults \n")
	checkResults(input, output)
	fmt.Printf("after checkResults \n")
}
