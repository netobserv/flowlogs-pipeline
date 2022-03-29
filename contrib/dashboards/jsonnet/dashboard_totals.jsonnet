
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
  title="Flow-Logs to Metrics - Totals",
  time_from="now",
  tags=['flp','grafana','dashboard','total'],
)
.addPanel(
  singlestat.new(
    datasource='prometheus',
    title="Total bandwidth",
  )
  .addTarget(
    prometheus.target(
      expr='sum(rate(flp_egress_per_destination_subnet[1m]))',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 5,
    h: 5,
  }
)
.addPanel(
  barGaugePanel.new(
    datasource='prometheus',
    title="Mice-elepahnts histogram",
  )
  .addTarget(
    prometheus.target(
      expr='flp_mice_elephants_histogram_bucket',
      format='heatmap',
      legendFormat='{{le}}',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 12,
    h: 8,
  }
)
.addPanel(
  singlestat.new(
    datasource='prometheus',
    title="Number of network services",
  )
  .addTarget(
    prometheus.target(
      expr='count(flp_service_count)',
    )
  ), gridPos={
    x: 0,
    y: 0,
    w: 5,
    h: 5,
  }
)