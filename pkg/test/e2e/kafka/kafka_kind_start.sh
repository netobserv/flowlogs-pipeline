#!/bin/bash

export KIND=../../../../bin/kind-v0.11.1

echo "KIND = " $KIND

$KIND create cluster
kubectl cluster-info --context kind-kind

echo
echo "Installing Kafka"
echo
kubectl apply -f ./strimzi.yaml -n default
sleep 60
kubectl apply -f ./kafka.strimzi.yaml


