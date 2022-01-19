#!/bin/bash

# doc link:: https://docs.openshift.com/container-platform/4.9/networking/ovn_kubernetes_network_provider/tracking-network-flows.html

set -eou pipefail

IPFIX_COLLECTOR_IP=${1-notset}

if ! [[ $IPFIX_COLLECTOR_IP =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo -e "\nUsage: $0 ipfix-collector-ip\n"
  exit 0
fi

enable-flow-export() {
  oc patch network.operator cluster --type='json' --patch='[{"op": "add","path": "/spec","value": {"exportNetworkFlows": {"ipfix": { "collectors": ["'"$IPFIX_COLLECTOR_IP"':2055"]}}}}]'
}

verify-flow-export() {
  oc get network.operator cluster -o jsonpath="{.spec.exportNetworkFlows}"
}

main() {
  echo -e "\nEnable flow export to network flows collector with IP $IPFIX_COLLECTOR_IP\n"
  enable-flow-export
  echo -e "\nverify::\n"
  verify-flow-export
  echo -e "\n\nNote: To remove all network flow collectors use:\n oc patch network.operator cluster --type='json' -p='[{\"op\":\"remove\", \"path\":\"/spec/exportNetworkFlows\"}]'"
}

main
