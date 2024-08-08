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
EOF
