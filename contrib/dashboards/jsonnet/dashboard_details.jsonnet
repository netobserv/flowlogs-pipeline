
local grafana = import 'grafana.libsonnet';
local dashboard = grafana.dashboard;
local row = grafana.row;
local singlestat = grafana.singlestat;
local graphPanel = grafana.graphPanel;
local heatmapPanel = grafana.heatmapPanel;
local barGaugePanel = grafana.barGaugePanel;
local table = grafana.table;
local prometheus = grafana.prometheus;
local template = grafana.template;

dashboard.new(
  schemaVersion=16,
  title="Flow-Logs to Metrics - Details",
  time_from="now-15m",
  tags=['flp','grafana','dashboard','details'],
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Bandwidth per network service",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_bandwidth_per_network_service[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Bandwidth per src and destination subnet",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_bandwidth_per_source_destination_subnet[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Bandwidth per source subnet",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_bandwidth_per_source_subnet[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  heatmapPanel.new(
    datasource='prometheus',
    title="Connection size in bytes heatmap",
    dataFormat="tsbuckets",
  )
  .addTarget(
    prometheus.target(
      expr='sum(rate(flp_connection_size_histogram_bucket[$__interval])) by (le)',
      format='heatmap',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 8,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Connections rate per destinationIP /16 subnets",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_connections_per_destination_subnet{_RecordType="newConnection"}[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Connections rate per sourceIP /16 subnets",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_connections_per_source_subnet{_RecordType="newConnection"}[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Connections rate per TCPFlags",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_connections_per_tcp_flags[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Connections rate per destination AS",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_connections_per_destination_as{_RecordType="newConnection"}[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Connections rate per source AS",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_connections_per_source_as{_RecordType="newConnection"}[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Connections rate of src / destination subnet occurences",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_count_per_source_destination_subnet{_RecordType="newConnection"}[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Bandwidth per destination subnet",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_egress_per_destination_subnet[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Bandwidth per namespace",
  )
  .addTarget(
    prometheus.target(
      expr='rate(flp_egress_per_namespace{aggregate=~".*Pod.*"}[1m])',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  heatmapPanel.new(
    datasource='prometheus',
    title="Flows length heatmap",
    dataFormat="tsbuckets",
  )
  .addTarget(
    prometheus.target(
      expr='sum(rate(flp_flows_length_histogram_bucket[$__interval])) by (le)',
      format='heatmap',
      legendFormat='{{le}}',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 8,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Connections rate per destinationIP geo-location",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_connections_per_destination_location{_RecordType="newConnection"}[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='loki',
    title="Bandwidth per source namespace",
  )
  .addTarget(
    prometheus.target(
      expr='sum by (srcK8S_Namespace) (sum_over_time({job="flowlogs-pipeline"} | json | unwrap bytes |  __error__="" [1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='loki',
    title="Loki logs rate",
  )
  .addTarget(
    prometheus.target(
      expr='rate({job="flowlogs-pipeline"}[60s])',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)
.addPanel(
  graphPanel.new(
    datasource='prometheus',
    title="Network services connections rate",
  )
  .addTarget(
    prometheus.target(
      expr='topk(10,rate(flp_service_count{_RecordType="newConnection"}[1m]))',
      legendFormat='',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 25,
    h: 20,
  }
)