#!/bin/bash

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


