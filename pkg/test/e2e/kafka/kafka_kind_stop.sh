#!/bin/bash

export KIND=../../../../bin/kind-v0.11.1

echo "delete kind cluster"
$KIND delete cluster --name test
