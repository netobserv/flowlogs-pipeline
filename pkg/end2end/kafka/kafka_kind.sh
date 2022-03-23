#!/bin/bash
set -x
export GOBIN=../../../bin
export KIND=$GOBIN/kind-v0.11.1

echo "GOBIN = " $GOBIN
echo "KIND = " $KIND

$KIND create cluster
kubectl cluster-info --context kind-kind

echo
echo "Installing Kafka"
echo
kubectl apply -f ./strimzi.yaml -n default
kubectl apply -f ./kafka.strimzi.yaml

echo "wait for kafka cluster to be active"
kubectl wait kafka/my-cluster --for=condition=Ready --timeout=1200s -n default
kubectl get kafka my-cluster -o=jsonpath='{.status.listeners[?(@.type=="external")].bootstrapServers}{"\n"}'
KAFKA_BROKER=$(kubectl get kafka my-cluster -o=jsonpath='{.status.listeners[?(@.type=="external")].bootstrapServers}{"\n"}')
export $KAFKA_BROKER
echo "KAFKA_BROKER = " $KAFKA_BROKER

export INPUT_FILE=../../../hack/examples/ocp-ipfix-flowlogs.json
echo "INPUT_FILE = " $INPUT_FILE
echo "running test"
go run kafka_end_to_end.go

echo "delete kind cluster"
$KIND delete cluster
