#!/bin/bash
echo "======> running confgenerator"
/app/confgenerator --srcFolder /app/network_definitions_custom \
                   --destConfFile /app/flowlogs-pipeline.conf.yaml
echo "======> running flowlogs-pipeline"
/app/flowlogs-pipeline --config /app/flowlogs-pipeline.conf.yaml
