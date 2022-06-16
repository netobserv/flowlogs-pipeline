#!/bin/bash

set -eou pipefail

PRE_REQ=$(cat <<-END
      echo "=== Installing pre-requisites  ===";
      apt update;
      apt install -y netbase;
      apt install -y curl;
      apt install -y net-tools;
      apt install -y ftp;
      apt install -y iperf3;
END
)

COMMON_POD_YAML=$(cat <<-END
    image: ubuntu
    imagePullPolicy: Always
    command: ["/bin/bash"]
END
)

create-flowlogs-pipeline-conf-file() {
  echo "====> Creating flowlogs-pipeline conf file (/tmp/flowlogs-pipeline.conf.yaml)"
  cat > /tmp/flowlogs-pipeline.conf.yaml <<EOL
log-level: error
pipeline:
  ingest:
    collector:
      hostname: 0.0.0.0
      port: 2055
      portLegacy: 2056
    type: collector
  encode:
    type: none
  extract:
    type: none
  transform:
  - generic:
      rules:
      - input: SrcAddr
        output: srcIP
      - input: SrcPort
        output: srcPort
      - input: DstAddr
        output: dstIP
      - input: DstPort
        output: dstPort
      - input: Proto
        output: proto
      - input: Bytes
        output: bytes
      - input: SequenceNum
        output: sequenceNum
      - input: Packets
        output: packets
    type: generic
  - network:
      rules:
      - input: srcIP
        output: srcK8S
        type: add_kubernetes
        parameters: ""
      - input: dstIP
        output: dstK8S
        type: add_kubernetes
        parameters: ""
    type: network
  write:
    type: stdout
EOL
}

deploy-ingress-workload() {
  NAMESPACE=ingress-workload
  POD_NAME=ingress
  SLEEP=120
  echo "====> Creating $NAMESPACE namespace"
  oc create namespace $NAMESPACE || true
  oc project $NAMESPACE || true
  echo "====> Deploying $POD_NAME pod"
  oc --ignore-not-found=true delete pod $POD_NAME
  oc create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  labels:
    app: $POD_NAME
spec:
  containers:
  - name: $POD_NAME
$COMMON_POD_YAML
    args:
    - -c
    - >
$PRE_REQ
      cd /tmp;
      echo "=== starting network-workload ===";
      while true; do
        echo "Downloading (from Latvia, Riga)";
        curl http://bks4-speedtest-1.tele2.net/10MB.zip -o /dev/null;
        echo "Done downloading.";
        echo "Waiting $SLEEP seconds";
        sleep $SLEEP;
        echo "Done waiting.";
      done ;
      echo "=== Exiting ===";
EOF
  oc project default || true
}

deploy-egress-workload() {
  NAMESPACE=egress-workload
  POD_NAME=egress
  SLEEP=120
  echo "====> Creating $NAMESPACE namespace"
  oc create namespace $NAMESPACE || true
  oc project $NAMESPACE || true
  echo "====> Deploying $POD_NAME pod"
  oc --ignore-not-found=true delete pod $POD_NAME
  oc create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  labels:
    app: $POD_NAME
spec:
  containers:
  - name: $POD_NAME
$COMMON_POD_YAML
    args:
    - -c
    - >
$PRE_REQ
      cd /tmp;
      fallocate -l 20M /tmp/20MB.zip;
      echo "=== starting network-workload ===";
      while true; do
        echo "Uploading (to Latvia, Riga)";
        curl -T /tmp/20MB.zip http://bks4-speedtest-1.tele2.net/upload.php -O /dev/null || true;
        echo "Done uploading.";
        echo "Waiting $SLEEP seconds";
        sleep $SLEEP;
        echo "Done waiting.";
      done ;
      echo "=== Exiting ===";
EOF
  oc project default || true
}

deploy-pod-to-pod-workload() {
  NAMESPACE=pod-to-pod-workload
  POD_NAME=iperf-server
  SLEEP=120
  echo "====> Creating $NAMESPACE namespace"
  oc create namespace $NAMESPACE || true
  oc project $NAMESPACE || true
  echo "====> Deploying $POD_NAME pod"
  oc --ignore-not-found=true delete pod $POD_NAME
  oc create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  labels:
    app: $POD_NAME
spec:
  containers:
  - name: $POD_NAME
$COMMON_POD_YAML
    args:
    - -c
    - >
$PRE_REQ
      echo "=== starting iperf3 server ===";
      iperf3 -s -p 3000 -f K
      done ;
      echo "=== Exiting ===";
EOF
  sleep 10
  IPERF_SERVER_IP=$(oc get -n $NAMESPACE pod $POD_NAME -o jsonpath="{.status.podIP}")
  echo "iperf server ip is $IPERF_SERVER_IP"

  POD_NAME=iperf-client
  echo "====> Deploying $POD_NAME pod"
  oc --ignore-not-found=true delete pod $POD_NAME
  oc create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  labels:
    app: $POD_NAME
spec:
  containers:
  - name: $POD_NAME
$COMMON_POD_YAML
    args:
    - -c
    - >
$PRE_REQ
      cd /tmp;
      echo "=== starting iperf client ===";
      while true; do
        echo "Stressing with iperf (20 seconds - 1 Mb/Sec)";
        iperf3 -c $IPERF_SERVER_IP -p 3000 -f K -t 20 -b 1M;
        echo "Done.";
        echo "Waiting $SLEEP seconds";
        sleep $SLEEP;
        echo "Done waiting.";
      done ;
      echo "=== Exiting ===";
EOF
  oc project default || true
}

deploy-flowlogs-pipeline() {
  create-flowlogs-pipeline-conf-file
  echo "====> Deploying flowlogs-pipeline project"
  oc project default || true
  export FLP_CONF_FILE=/tmp/flowlogs-pipeline.conf.yaml
  make ocp-deploy
}

main() {
  echo ":::====> Start executing"
  deploy-ingress-workload
  deploy-egress-workload
  deploy-pod-to-pod-workload
  deploy-flowlogs-pipeline
  echo ":::====> Done executing"
  POD_NAME=$(oc get pod -n default -l app=flowlogs-pipeline -o jsonpath="{.items[0].metadata.name}")
  echo "Use: kubectl logs $POD_NAME  | grep ingress-workload"
  echo "Use: kubectl logs $POD_NAME  | grep egress-workload"
  echo "Use: kubectl logs $POD_NAME  | grep pod-to-pod-workload"
}

main
