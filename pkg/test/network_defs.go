package test

const ConfgenShortConfig = `#flp_confgen
description:
  test description
ingest:
  collector:
    port: 2155
    portLegacy: 2156
    hostName: 0.0.0.0
encode:
  type: prom
  prom:
    prefix: flp_
visualization:
  grafana:
    dashboards:
      - name: "details"
        title: "Flow-Logs to Metrics - Details"
        time_from: "now-15m"
        tags: "['flp','grafana','dashboard','details']"
        schemaVersion: "16"
`

const ConfgenLongConfig = `#flp_confgen
description:
  test description
ingest:
  collector:
    port: 2155
    portLegacy: 2156
    hostName: 0.0.0.0
transform:
  generic:
    rules:
    - input: SrcAddr
      output: srcIP
encode:
  type: prom
  prom:
    prefix: flp_
write:
  type: loki
  loki:
    url: http://loki:3100
    staticLabels:
      job: flowlogs-pipeline
visualization:
  grafana:
    dashboards:
      - name: "details"
        title: "Flow-Logs to Metrics - Details"
        time_from: "now-15m"
        tags: "['flp','grafana','dashboard','details']"
        schemaVersion: "16"
`

const ConfgenNetworkDefBase = `#flp_confgen
description:
  test description
details:
  test details
usage:
  test usage
tags:
  - test
  - label
transform:
  rules:
    - type: add_service
      add_service:
        input: testInput
        output: testOutput
        protocol: proto
extract:
  types: aggregates
  aggregates:
    rules:
      - name: test_aggregates
        groupByKeys:
          - service
        operationType: sum
        operationKey: test_operation_key
encode:
  type: prom
  prom:
    metrics:
      - name: test_metric
        type: gauge
        valueKey: test_aggregates_value
        labels:
          - groupByKeys
          - aggregate
        filters:
          - key: K
            value: V
visualization:
  type: grafana
  grafana:
  - expr: 'test expression'
    type: graphPanel
    dashboard: details
    title:
      Test grafana title
`

const ConfgenNetworkDefHisto = `#flp_confgen
description:
  test description
details:
  test details
usage:
  test usage
tags:
  - test
  - label
extract:
  type: aggregates
  aggregates:
    rules:
      - name: test_agg_histo
        groupByKeys:
          - service
        operationType: sum
        operationKey: test_operation_key
encode:
  type: prom
  prom:
    metrics:
      - name: test_histo
        type: histogram
        valueKey: test_aggregates_value
        labels:
          - groupByKeys
          - aggregate
        filters:
          - key: K
            value: V
`

const ConfgenNetworkDefNoAgg = `#flp_confgen
description:
  test description
details:
  test details
usage:
  test usage
tags:
  - test
  - label
transform:
  rules:
    - type: add_service
      add_service:
        input: testInput
        output: testOutput
        protocol: proto
encode:
  type: prom
  prom:
    metrics:
      - name: test_metric
        type: counter
        valueKey: Bytes
        labels:
          - service
        filters:
          - key: K
            value: V
visualization:
  type: grafana
  grafana:
  - expr: 'test expression'
    type: graphPanel
    dashboard: details
    title:
      Test grafana title
`
