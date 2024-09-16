# Configuration generator (a.k.a confGenerator)

confGenerator is a utility to generate from `metric definitions` provided by users 
as a set of files, folders and sub-folders the following:

1. flowlogs-pipeline configuration files
1. Visualization configuration (Grafana)
1. Metrics documentation 

The information from the (human friendly) `metric definitions` is aggregated and re-structured to 
generate output configurations and documentation automatically

## Usage:

```bash
$ ./confgenerator --help
Generate configuration and docs from metric definitions

Usage:
  confgenerator [flags]

Flags:
      --config string                     config file (default is $HOME/.confgen)
      --destConfFile string               destination configuration file (default "/tmp/flowlogs-pipeline.conf.yaml")
      --destDashboardFolder string        destination grafana dashboard folder (default "/tmp/dashboards")
      --destDocFile string                destination documentation file (.md) (default "/tmp/metrics.md")
      --destGrafanaJsonnetFolder string   destination grafana jsonnet folder (default "/tmp/jsonnet")
      --generateStages strings            Produce only specified stages (ingest, transform_generic, transform_network, extract_aggregate, encode_prom, write_loki
  -h, --help                              help for confgenerator
      --log-level string                  Log level: debug, info, warning, error (default "error")
      --skipWithTags strings              Skip definitions with Tags
      --srcFolder string                  source folder (default "network_definitions")
```

> Note: confgenerator is available also from `netobserv/flowlogs-pipeline` quay image. To use execute:  
> `docker run --entrypoint /app/confgenerator quay.io/netobserv/flowlogs-pipeline:main --help` 

> Note: The default location for network definitions in flowlogs-pipeline is `/network_definitions` folder

The files and folder structure required as input for `confGenerator` are:

1. `config.yaml` - singleton file holding general information for configuration generation   
1. `metric definition` files - each file represent one or more metrics including the documentation,
flowlogs-pipeline pipeline steps and visualization configuration

> Note: It is possible to place `metric definition` files in any sub-folder structure 

> Note: It is possible to activate `ConfGenrator` with the default flowlogs-pipeline configuration using the command `make generate-configuration`. 
> This command compiles the code and generates default outputs.

> Note: `Confgenerator` uses grafana libraries taken from `github.com/grafana/grafonnet-lib`, (commit 30280196507e0fe6fa978a3e0eaca3a62844f817).

## How to write a network definitions

This is easy and quick:

(1) create new definition yaml file or modify existing under the folder `network_definitions`.  
(2) execute  
```bash
make generate-configuration
make dashboards
```
(3) For `OCP` deployment execute 
```bash
make ocp-deploy
``` 
For `Kind` deployment execute 
```bash
make local-redeploy
```
> Note: Additional information on usage and deployment can be found in flowlogs-pipeline README  

> Note: Learning from examples and existing metric definitions is very useful.

### Network definition explained

In this section we explain how network definition is structured. This is useful for development of 
new network definitions as well as debugging and working with existing network definition.

```shell
#flp_confgen (1) 
description: (2)
  Network definition description  
details: (3)
  More details  
usage: (4)
  How to use
labels: (5) 
  - label1
  - label2
transform: (6)
  rules: (7)
    - input: inputFieldName (7.1)
      output: outputFieldName (7.2)
      type: operationType (7.3)
      parameters: operationParameters (7.4)
extract: (8)
  type: aggregates (8.1)
  aggregates:
    - name: aggregate_name (8.2)
      groupByKeys: (8.3)
        - aggregateField1
        - aggregateField2
      operationType: operation (8.4)
encode: (9)
  type: prom (9.1)
  prom:
    metrics:
      - name: metricName (9.2)
        type: metricType (9.3)
        filter: {key: myKey, value: myValue} (9.4)
        valueKey: value (9.5)
        labels: (9.6)
          - by
          - aggregate
visualization: (10)
  type: grafana
  grafana: 
    - expr: 'flp_metricName' (10.1)
      type: graphPanel (10.2)
      dashboard: dashboardName (10.3)
      title:
        grafanaPanelTitle (10.4)
```

(1) A fixed header `#flp_confgen` that must exist in every network definition file  
(2) (3) (4) description, details and usage of the definition file - used for docs  
(5) labels - used by visualization and docs    

(6) The transform phase defines what network fields to add on top of the original log fields.
This is done using multiple rules (7). In each of the rules the input field (7.1) is being 
evaluated with network operation (7.3) to generate a new field (7.2) with network data.
As needed, (7.4) adds additional parameters to the operation. 
> For additional details on `network transform` including the list of supported operations, 
> refer to [README.md](../README.md#transform-network) and for additional details 
> on the fields refer to [api.md](api.md#transform-network-api). 

(8) Next, the transformed log lines are aggregated using **mathematical 
operation** (8.4) based on the `groupByKeys` fields (8.3) - 
this actually moves the data from being log lines into being a metric named (8.2)  
> For additional details on `extract aggregates`
> refer to [README.md](../README.md#aggregates).  

(9) Next, the metrics from (8.2) are sent to prometheus (9.1). <br>
The metric name in prometheus will be called as the value of (9.2) with 
the prefix from the `config.yaml` file. <br>
The type of the prometheus metric will be (9.3) (e.g. gauge, counter or histogram). <br>
The filter field (9.4) determines which aggregates will be taken into account. <br>
The key should be `"name"` and the value should match the aggregate name (8.2). <br>
The value to be used by prometheus is taken from the field defined in (9.5). <br>
For `Gauge`, use `total_value` or `total_count`. <br>
For `Counter`, use `recent_op_value` or `recent_count`. <br>
For `Histogram`, use `recent_raw_values`. <br>
Prometheus will add labels to the metric based on the (9.6) fields. <br>

(10) next, using grafana to visualize the metric with name from (9.2) including the 
prefix and using the expression from (10.1). 
Grafana will visualize the metric as (10.2) and place the panel inside
a dashboard named (10.3) as defined in `config.yaml`. 
The title for the panel will be (10.4)  

The type field for (10.2) can be one of:
"graphPanel", "singleStat", "barGauge", "heatmap" to use prometheus datasource and visualize accordingly or,  
"lokiGraphPanel" to use loki datasource and visualize accordingly

> [connection_rate_per_dest_subnet.yaml](../network_definitions/connection_rate_per_dest_subnet.yaml) is an
> example for a network_definition file in which the metric is defined to hold counts 
> for the number of connections per subnet and the visualization is defined to show 
> the top 10 metrics in a graph panel.



