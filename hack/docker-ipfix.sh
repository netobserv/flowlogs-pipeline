#!/usr/bin/env bash

name=ipfix-observ
lokiName=$name-loki
consoleName=$name-console

echo "Running $name using docker"
docker create --name $name --net=host quay.io/netobserv/flowlogs-pipeline:main --config=/root/config.yaml --log-level=debug
docker create --name $consoleName --net=host quay.io/netobserv/network-observability-standalone-frontend:main --config=/root/config.yaml --loglevel=trace
docker run -d --name $lokiName --net=host grafana/loki

docker cp ./hack/examples/docker-ipfix-config.yaml  $name:/root/config.yaml
docker start $name

docker cp ./hack/examples/docker-console-config.yaml  $consoleName:/root/config.yaml
docker start $consoleName

stopAndRemove() {
  docker stop $1
  docker rm $1
}

exitFunc() {
  echo "Stopping $name..."
  stopAndRemove $name
  stopAndRemove $lokiName
  stopAndRemove $consoleName
  exit 0
}

trap exitFunc SIGINT SIGTERM
docker logs --follow $name