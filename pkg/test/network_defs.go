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
    port: 9102
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
    port: 9102
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
    - input: testInput
      output: testOutput
      type: add_service
      parameters: proto
extract:
  aggregates:
    - name: test_aggregates
      by:
        - service
      operation: sum
      recordKey: test_record_key
encode:
  type: prom
  prom:
    metrics:
      - name: test_metric
        type: gauge
        valueKey: test_aggregates_value
        labels:
          - by
          - aggregate
visualization:
  type: grafana
  grafana:
  - expr: 'test expression'
    type: graphPanel
    dashboard: test
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
  aggregates:
    - name: test_agg_histo
      by:
        - service
      operation: sum
      recordKey: test_record_key
encode:
  type: prom
  prom:
    metrics:
      - name: test_histo
        type: histogram
        valueKey: test_aggregates_value
        labels:
          - by
          - aggregate
`

const ConfgenNetworkDefNoAgg = `#flp_confgen
description:
  test description
details:
  test details
usage:
  test usage
labels:
  - test
  - label
transform:
  rules:
    - input: testInput
      output: testOutput
      type: add_service
      parameters: proto
encode:
  type: prom
  prom:
    metrics:
      - name: test_metric
        type: counter
        valueKey: Bytes
        labels:
          - service
visualization:
  type: grafana
  grafana:
  - expr: 'test expression'
    type: graphPanel
    dashboard: test
    title:
      Test grafana title
`
