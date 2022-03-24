#!/bin/bash

export GOBIN=../../../../bin
export KIND=$GOBIN/kind-v0.11.1

echo "delete kind cluster"
$KIND delete cluster
