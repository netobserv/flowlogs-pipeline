#!/bin/bash

export KIND=../../../../bin/kind-v0.11.1

echo "KIND = " $KIND

$KIND create cluster --name kafka-e2e-test
kubectl cluster-info --context kafka-e2e-test

echo
echo "Installing Kafka"
echo
kubectl apply -f ./strimzi.yaml -n default
kubectl apply -f ./kafka.strimzi.yaml

