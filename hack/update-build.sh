#!/usr/bin/env bash

echo "Updating container file"

: "${CONTAINER_FILE:=./contrib/docker/Dockerfile}"
: "${COMMIT:=$(git rev-list --abbrev-commit --tags --max-count=1)}"

cat <<EOF >>"${CONTAINER_FILE}"
LABEL com.redhat.component="network-observability-flowlogs-pipeline-container"
LABEL name="network-observability-flowlogs-pipeline"
LABEL io.k8s.display-name="Network Observability Flow-Logs Pipeline"
LABEL io.k8s.description="Network Observability Flow-Logs Pipeline"
LABEL summary="Network Observability Flow-Logs Pipeline"
LABEL maintainer="support@redhat.com"
LABEL io.openshift.tags="network-observability-flowlogs-pipeline"
LABEL upstream-vcs-ref="${COMMIT}"
LABEL upstream-vcs-type="git"
LABEL description="Flow-Logs Pipeline (a.k.a. FLP) is an observability tool that consumes logs from various inputs, transform them and export logs to loki and / or time series metrics to prometheus."
EOF


sed -i 's/\(FROM.*\)docker.io\/library\/golang:1.22\(.*\)/\1brew.registry.redhat.io\/rh-osbs\/openshift-golang-builder:v1.22.5-202407301806.g4c8b32d.el9\2/g' ${CONTAINER_FILE}
