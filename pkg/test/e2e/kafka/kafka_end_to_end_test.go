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

package test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline"
	kafkago "github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

const (
	defaultInputFile        = "../../../../hack/examples/ocp-ipfix-flowlogs.json"
	kafkaBrokerDefaultAddr  = "localhost:9092"
	inputFileEnvVar         = "INPUT_FILE"
	kafkaBrokerEnvVar       = "KAFKA_BROKER"
	kafkaInputTopicEnvVar   = "KAFKA_INPUT_TOPIC"
	kafkaOutputTopicEnvVar  = "KAFKA_OUTPUT_TOPIC"
	kafkaInputTopicDefault  = "topic_in"
	kafkaOutputTopicDefault = "topic_out"
)

type lineBuffer []byte

func makeKafkaProducer(t *testing.T) *kafkago.Writer {
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
	log.Debugf("DeleteTopics response = %v, err = %v \n", deleteResponse, err)
	fmt.Printf("wait a second for topic deletion to complete \n")
	time.Sleep(time.Second)

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
		Addr:  kafkago.TCP(kafkaAddr),
		Topic: topic,
	}
	return &kafkaProducer
}

func makeKafkaConsumer(t *testing.T) *kafkago.Reader {
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
	assert.NoError(t, err)
	log.Debugf("DeleteTopics response = %v, err = %v \n", deleteResponse, err)
	fmt.Printf("wait a second for topic deletion to complete \n")
	time.Sleep(time.Second)

	fmt.Printf("create topic: %s \n", topic)
	createTopcisResponse, err := kafkaClient.CreateTopics(context.Background(), &kafkago.CreateTopicsRequest{
		Topics: []kafkago.TopicConfig{
			{Topic: topic,
				NumPartitions:     1,
				ReplicationFactor: 1}},
	})
	assert.NoError(t, err)
	log.Debugf("createTopcisResponse response = %v, err = %v \n", createTopcisResponse, err)

	electResponse, err := kafkaClient.ElectLeaders(context.Background(), &kafkago.ElectLeadersRequest{
		Topic: topic,
	})
	assert.NoError(t, err)
	log.Debugf("ElectLeaders response = %v, err = %v \n", electResponse, err)

	// prepare a kafka consumer on specified topic
	kafkaConsumer := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     []string{kafkaAddr},
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

func receiveData(t *testing.T, consumer *kafkago.Reader, nLines int) []lineBuffer {
	fmt.Printf("receiveData:  nLines = %d \n", nLines)
	output := make([]lineBuffer, nLines)
	for i := 0; i < nLines; i++ {
		kafkaMessage, _ := consumer.ReadMessage(context.Background())
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

func setupPipeline(t *testing.T) {

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
	assert.NoError(t, err)
	if err != nil {
		msg := fmt.Sprintf("Error reading config file, err = %v \n", err)
		assert.Fail(t, msg)
	}

	var b []byte
	pipelineStr := v.Get("pipeline")
	b, err = json.Marshal(&pipelineStr)
	assert.NoError(t, err)
	if err != nil {
		msg := fmt.Sprintf("error marshaling: %v\n", err)
		assert.Fail(t, msg)
	}
	config.Opt.PipeLine = string(b)
	parametersStr := v.Get("parameters")
	b, err = json.Marshal(&parametersStr)
	assert.NoError(t, err)
	if err != nil {
		msg := fmt.Sprintf("error marshaling: %v\n", err)
		assert.Fail(t, msg)
	}
	config.Opt.Parameters = string(b)
	err = json.Unmarshal([]byte(config.Opt.PipeLine), &config.PipeLine)
	assert.NoError(t, err)
	if err != nil {
		msg := fmt.Sprintf("error unmarshaling: %v\n", err)
		assert.Fail(t, msg)
	}
	err = json.Unmarshal([]byte(config.Opt.Parameters), &config.Parameters)
	assert.NoError(t, err)
	if err != nil {
		msg := fmt.Sprintf("error unmarshaling: %v\n", err)
		assert.Fail(t, msg)
	}

	err = config.ParseConfig()
	assert.NoError(t, err)
	if err != nil {
		msg := fmt.Sprintf("error in parsing config file: %v \n", err)
		assert.Fail(t, msg)
	}

	var mainPipeline *pipeline.Pipeline
	mainPipeline, _ = pipeline.NewPipeline()

	// run the pipeline in a separate go-routine
	go func() {
		mainPipeline.Run()
	}()
}

func runCommand(t *testing.T, command string) {
	var cmd *exec.Cmd
	cmdStrings := strings.Split(command, " ")
	cmdBase := cmdStrings[0]
	cmdStrings = cmdStrings[1:]
	cmd = exec.Command(cmdBase, cmdStrings...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	assert.NoError(t, err)
	if err != nil {
		msg := fmt.Sprintf("error in running command: %v \n", err)
		assert.Fail(t, msg)
	}
}

func runCommandGetOutput(t *testing.T, command string) string {
	var cmd *exec.Cmd
	var outBuf bytes.Buffer
	var err error
	cmdStrings := strings.Split(command, " ")
	cmdBase := cmdStrings[0]
	cmdStrings = cmdStrings[1:]
	cmd = exec.Command(cmdBase, cmdStrings...)
	cmd.Stdout = &outBuf
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	assert.NoError(t, err)
	if err != nil {
		msg := fmt.Sprintf("error in running command: %v \n", err)
		assert.Fail(t, msg)
	}
	output := outBuf.Bytes()
	// strip the line feed from the end of the output string
	output = output[0 : len(output)-1]
	fmt.Printf("output = %s\n", string(output))
	return string(output)
}

func TestEnd2EndKafka(t *testing.T) {
	var command string

	pwd := runCommandGetOutput(t, "pwd")

	fmt.Printf("\nset up kind and kafka \n\n")
	command = pwd + "/kafka_kind_start.sh"
	runCommand(t, command)

	fmt.Printf("\nwait for kafka to be active \n\n")
	command = "kubectl wait kafka/my-cluster --for=condition=Ready --timeout=1200s -n default"
	runCommand(t, command)

	command = "kubectl get kafka my-cluster -o=jsonpath='{.status.listeners[?(@.type==\"external\")].bootstrapServers}{\"\\n\"}'"
	kafkaAddr := runCommandGetOutput(t, command)
	// strip the quotation marks
	kafkaAddr = kafkaAddr[1 : len(kafkaAddr)-1]
	fmt.Printf("kafkaAddr = %s \n", kafkaAddr)
	err := os.Setenv(kafkaBrokerEnvVar, kafkaAddr)
	assert.NoError(t, err)
	if err != nil {
		msg := fmt.Sprintf("error in Setenv: %v \n", err)
		assert.Fail(t, msg)
	}

	fmt.Printf("create kafka producer \n")
	producer := makeKafkaProducer(t)
	fmt.Printf("create kafka consumer \n")
	consumer := makeKafkaConsumer(t)
	fmt.Printf("set up pipeline \n")
	setupPipeline(t)
	fmt.Printf("read input \n")
	input := getInput(t)
	nLines := len(input)
	fmt.Printf("send input data to kafka input topic \n")
	sendKafkaData(t, producer, input)
	fmt.Printf("read data from kafka output topic \n")
	output := receiveData(t, consumer, nLines)
	fmt.Printf("check results \n")
	checkResults(t, input, output)

	fmt.Printf("delete kind and kafka \n")
	command = pwd + "/kafka_kind_stop.sh"
	runCommand(t, command)
}
