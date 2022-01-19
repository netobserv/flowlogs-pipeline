#!/bin/bash

set -eou pipefail

JB=jb-v0.4.0
JSONNET=jsonnet-v0.17.0

generate-dashboards() {
	mkdir -p bin/grafonnet
	cd bin/grafonnet
	$JB init  > /dev/null 2>&1 || true
	$JB install https://github.com/grafana/grafonnet-lib/grafonnet || true
	cd ../../contrib/dashboards
	for jsonnet_file in jsonnet/*.jsonnet; do
	  filename="$(basename -- "${jsonnet_file%.*}")"
	  $JSONNET -J ../../bin/grafonnet/vendor/github.com/grafana/grafonnet-lib/grafonnet "$jsonnet_file" > "$filename".json || true
	done
}

main() {
  cwd=$(pwd)
  generate-dashboards
  cd "$cwd"
}

main
