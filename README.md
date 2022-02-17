
[![pull request](https://github.com/netobserv/flowlogs2metrics/actions/workflows/pull_request.yml/badge.svg?branch=main)](https://github.com/netobserv/flowlogs2metrics/actions/workflows/pull_request.yml)
[![Push image to quay.io](https://github.com/netobserv/flowlogs2metrics/actions/workflows/push_image.yml/badge.svg)](https://github.com/netobserv/flowlogs2metrics/actions/workflows/push_image.yml)
[![codecov](https://codecov.io/gh/netobserv/flowlogs2metrics/branch/main/graph/badge.svg?token=KMZKG6PRS9)](https://codecov.io/gh/netobserv/flowlogs2metrics)

# Overview

**Flow-Logs to Metrics** (a.k.a. FL2M) is an **observability tool** that consumes raw **network flow-logs** and
transforms them from their original format (NetFlow or IPFIX) into **prometheus numeric metrics** format.
FL2M allows to define mathematical transformations to generate condense metrics that encapsulate network domain knowledge.

In addition, FL2M decorates the metrics with **context**, allowing visualization layers and analytics frameworks to present **network insights** to SRE’s, cloud operators and network experts.

Along with Prometheus and its ecosystem tools such as Thanos, Cortex etc., FL2M provides efficient scalable multi-cloud solution for comprehensive network analytics that rely **solely based on metrics data-source**.

Default metrics are documented here [docs/metrics.md](docs/metrics.md).
<br>
<br>

> Note: prometheus eco-system tools such as Alert Manager can be used with FL2M to generate alerts and provide big-picture insights.


![Data flow](docs/images/data_flow.drawio.png "Data flow")

# Usage

<!---AUTO-flowlogs2metrics_help--->
```bash
Expose network flow-logs from metrics  
  
Usage:  
  flowlogs2metrics [flags]  
  
Flags:  
      --config string                          config file (default is $HOME/.flowlogs2metrics)  
  -h, --help                                   help for flowlogs2metrics  
      --log-level string                       Log level: debug, info, warning, error (default "error")  
      --pipeline.decode.aws string             aws fields  
      --pipeline.decode.type string            Decode type: aws, json, none  
      --pipeline.encode.kafka string           Kafka encode API  
      --pipeline.encode.prom string            Prometheus encode API  
      --pipeline.encode.type string            Encode type: prom, json, kafka, none  
      --pipeline.extract.aggregates string     Aggregates (see docs)  
      --pipeline.extract.type string           Extract type: aggregates, none  
      --pipeline.ingest.collector string       Ingest collector API  
      --pipeline.ingest.file.filename string   Ingest filename (file)  
      --pipeline.ingest.kafka string           Ingest Kafka API  
      --pipeline.ingest.type string            Ingest type: file, collector,file_loop (required)  
      --pipeline.transform string              Transforms (list) API (default "[{"type": "none"}]")  
      --pipeline.write.loki string             Loki write API  
      --pipeline.write.type string             Write type: stdout, none
```
<!---END-AUTO-flowlogs2metrics_help--->

> Note: for API details refer to  [docs/api.md](docs/api.md).
> 
## Configuration generation

flowlogs2metrics network metrics configuration ( `--config` flag) can be generated automatically using 
the `confGenerator` utility. `confGenerator` aggregates information from multiple user provided network *metric 
definitions* into flowlogs2metrics configuration. More details on `confGenerator` can be found 
in [docs/confGenrator.md](docs/confGenerator.md).
 
To generate flowlogs2metrics configuration execute:  
```shell
make generate-configuration
make dashboards
```

## Deploy into OpenShift (OCP)
To deploy FL2M on OCP perform the following steps:
1. Deploy OCP and make sure `kubectl` works with the cluster
```shell
kubectl get namespace openshift
```
2. Deploy FL2M (into `default` namespace)
```shell
kubectl config set-context --current --namespace=default
make deploy
```
3. Enable export OCP flowlogs into FL2M
```shell
flowlogs2metrics_svc_ip=$(kubectl get svc flowlogs2metrics -o jsonpath='{.spec.clusterIP}')
./hack/enable-ocp-flow-export.sh $flowlogs2metrics_svc_ip
```
4. Verify flowlogs are captured
```shell
kubectl logs -l app=flowlogs2metrics -f
```

## Deploy with Kind and netflow-simulator (for development and exploration)
These instructions apply for deploying FL2M development and exploration environment with [kind](https://kind.sigs.k8s.io/) and [netflow-simulator](https://hub.docker.com/r/networkstatic/nflow-generator),
tested on Ubuntu 20.4 and Fedora 34.
1. Make sure the following commands are installed and can be run from the current shell:
   - make
   - go (version 1.17)
   - docker
1. To deploy the full simulated environment which includes a kind cluster with FL2M, Prometheus, Grafana, and
   netflow-simulator, run (note that depending on your user permissions, you may have to run this command under sudo):
    ```shell
    make local-deploy
    ````
   If the command is successful, the metrics will get generated and can be observed by running (note that depending
   on your user permissions, you may have to run this command under sudo):
    ```shell
    kubectl logs -l app=flowlogs2metrics -f
    ```
    The metrics you see upon deployment are default and can be modified through configuration described [later](#Configuration).

# Technology

FL2M is a framework. The main FL2M object is the **pipeline**. FL2M **pipeline** can be configured (see
[Configuration section](#Configuration)) to extract the flow-log records from a source in a standard format such as NetFLow or IPFIX, apply custom processing, and output the result as metrics (e.g., in Prometheus format).

# Architecture

The pipeline is constructed of a sequence of stages:
- **ingest** - obtain flows from some source, one entry per line
- **decode** - parse input lines into a known format, e.g., dictionary (map) of AWS or goflow data
- **transform** - convert entries into a standard format; can include multiple transform stages
- **write** - provide the means to write the data to some target, e.g. loki, standard output, etc
- **extract** - derive a set of metrics from the imported flows
- **encode** - make the data available in appropriate format (e.g. prometheus)

The **encode** and **write** stages may be combined in some cases (as in prometheus), in which case **write** is set to **none**

It is expected that the **ingest** module will receive flows every so often, and this ingestion event will then trigger the rest of the pipeline. So, it is the responsibility of the **ingest** module to provide the timing of when (and how often) the pipeline will run.

# Configuration

It is possible to configure flowlogs2metrics using command-line-parameters, configuration file, environment variables, or any combination of those options.

For example:
1. Using command line parameters:
```./flowlogs2metrics --pipeline.ingest.type file --pipeline.ingest.file.filename hack/examples/ocp-ipfix-flowlogs.json```
2. Using configuration file:
- create under $HOME/.flowlogs2metrics.yaml the file:
```yaml
pipeline:
  ingest:
    type: file
    file:
      filename: hack/examples/ocp-ipfix-flowlogs.json
```
- execute `./flowlogs2metrics`
3. using environment variables:
- set environment variables
```bash
export FLOWLOGS2METRICS_PIPELINE_INGEST_TYPE=file
export FLOWLOGS2METRICS_PIPELINE_INGEST_FILE_FILENAME=hack/examples/ocp-ipfix-flowlogs.json
```
- execute `./flowlogs2metrics`

# Syntax of portions of the configuration file

## Supported stage types

Supported options and stage types are provided by running:

```
flowlogs2metrics --help
```

### Transform
Different types of inputs come with different sets of keys.
The transform stage allows changing the names of the keys and deriving new keys from old ones.
Multiple transforms may be specified, and they are applied in the **order of specification**.
The output from one transform becomes the input to the next transform.

### Transform Generic

The generic transform module maps the input json keys into another set of keys.
This allows to perform subsequent operations using a uniform set of keys.
In some use cases, only a subset of the provided fields are required.
Using the generic transform, we may specify those particular fields that interest us.

For example, suppose we have a flow log with the following syntax:
```
{"Bytes":20800,"DstAddr":"10.130.2.2","DstPort":36936,"Packets":400,"Proto":6,"SequenceNum":1919,"SrcAddr":"10.130.2.13","SrcHostIP":"10.0.197.206","SrcPort":3100,"TCPFlags":0,"TimeFlowStart":0,"TimeReceived":1637501832}
```

Suppose further that we are only interested in fields with source/destination addresses and ports, together with bytes and packets transferred.
The yaml specification for these parameters would look like this:

```yaml
pipeline:
  transform:
    - type: generic
      generic:
        rules:
        - input: Bytes
          output: bytes
        - input: DstAddr
          output: dstAddr
        - input: DstPort
          output: dstPort
        - input: Packets
          output: packets
        - input: SrcAddr
          output: srcAddr
        - input: SrcPort
          output: srcPort
        - input: TimeReceived
          output: timestamp
```

Each field specified by `input` is translated into a field specified by the corresponding `output`.
Only those specified fields are saved for further processing in the pipeline.
Further stages in the pipeline should use these new field names.
This mechanism allows us to translate from any flow-log layout to a standard set of field names.
If the `input` and `output` fields are identical, then that field is simply passed to the next stage.
For example:
```yaml
pipeline:
  transform:
    - type: generic
      generic:
        rules:
        - input: DstAddr
          output: dstAddr
        - input: SrcAddr
          output: srcAddr
    - type: generic
      generic:
        rules:
        - input: dstAddr
          output: dstIP
        - input: dstAddr
          output: dstAddr
        - input: srcAddr
          output: srcIP
        - input: srcAddr
          output: srcAddr
```
Before the first transform suppose we have the keys `DstAddr` and `SrcAddr`.
After the first transform, we have the keys `dstAddr` and `srcAddr`.
After the second transform, we have the keys `dstAddr`, `dstIP`, `srcAddr`, and `srcIP`.

### Transform Network

`transform network` provides specific functionality that is useful for transformation of network flow-logs:

1. Resolve subnet from IP addresses
1. Resolve known network service names from port numbers and protocols
1. Perform simple mathematical transformations on field values
1. Compute geo-location from IP addresses
1. Resolve kubernetes information from IP addresses
1. Perform regex operations on field values

Example configuration:

```yaml
pipeline:
  transform:
    - type: network
      network:
        KubeConfigPath: /tmp/config
        rules:
        - input: srcIP
          output: srcSubnet
          type: add_subnet
          parameters: /24
        - input: value
          output: value_smaller_than10
          type: add_if
          parameters: <10
        - input: dstPort
          output: service
          type: add_service
          parameters: protocol
        - input: dstIP
          output: dstLocation
          type: add_location
        - input: srcIP
          output: srcK8S
          type: add_kubernetes
        - input: srcSubnet
          output: match-10.0
          type: add_regex_if
          parameters: 10.0.*
        - input: "{{.srcIP}},{{.srcPort}},{{.dstIP}},{{.dstPort}},{{.protocol}}"
          output: isNewFlow
          type: conn_tracking
          parameters: "1"
```

The first rule `add_subnet` generates a new field named `srcSubnet` with the 
subnet of `srcIP` calculated based on prefix length from the `parameters` field 

The second `add_if` generates a new field named `value_smaller_than10` that contains 
the contents of the `value` field for entries that satisfy the condition specified 
in the `parameters` variable (smaller than 10 in the example above). In addition, the
field `value_smaller_than10_Evaluate` with value `true` is added to all satisfied
entries

The third rule `add_service` generates a new field named `service` with the known network 
service name of `dstPort` port and `protocol` protocol. Unrecognized ports are ignored 
> Note: `protocol` can be either network protocol name or number

The fourth rule `add_location` generates new fields with the geo-location information retrieved 
from DB [ip2location](https://lite.ip2location.com/) based on `dstIP` IP. 
All the geo-location fields will be named by appending `output` value 
(`dstLocation` in the example above) to their names in the [ip2location](https://lite.ip2location.com/ DB 
(e.g., `CountryName`, `CountryLongName`, `RegionName`, `CityName` , `Longitude` and `Latitude`)

The fifth rule `add_kubernetes` generates new fields with kubernetes information by
matching the `input` value (`srcIP` in the example above) with k8s `nodes`, `pods` and `services` IPs.
All the kubernetes fields will be named by appending `output` value
(`srcK8S` in the example above) to the kubernetes metadata field names
(e.g., `Type`, `Name` and `Namespace`)
> Note: kubernetes connection is done using the first available method: 
> 1. configuration parameter `KubeConfigPath` (in the example above `/tmp/config`) or
> 2. using `KUBECONFIG` environment variable
> 3. using local `~/.kube/config`

The sixth rule `add_regex_if` generates a new field named `match-10.0` that contains
the contents of the `srcSubnet` field for entries that match regex expression specified 
in the `parameters` variable. In addition, the field `match-10.0_Matched` with 
value `true` is added to all matched entries

The seventh rule `conn_tracking` generates a new field named `isNewFlow` that contains
the contents of the `parameters` variable **only for new entries** (first seen in 120 seconds) 
that match hash of template fields from the `input` variable. 


> Note: above example describes all available transform network `Type` options
> Note: above transform is essential for the `aggregation` phase  

### Aggregates

Aggregates are used to define the transformation of flow-logs from textual/json format into
numeric values to be exported as metrics. Aggregates are dynamically created based
on defined values from fields in the flow-logs and on mathematical functions to be performed
on these values.
The specification of the aggregates details is placed in the `extract` stage of the pipeline.

For Example, assuming set of flow-logs, with single sample flow-log that looks like:
```
{"srcIP":   "10.0.0.1",
"dstIP":   "20.0.0.2",
"level":   "error",
"value":   "7",
"message": "test message"}
```

It is possible to define aggregates per `srcIP` or per `dstIP` of per the tuple `srcIP`x`dstIP`
to capture the `sum`, `min`, `avg` etc. of the values in the field `value`.

For example, configuration record for aggregating field `value` as
average for `srcIP`x`dstIP` tuples will look like this::

```yaml
pipeline:
  extract:
    type: aggregates
    aggregates:
    - Name: "Average key=value for (srcIP, dstIP) pairs"
      By:
      - "dstIP"
      - "srcIP"
      Operation: "avg"
      RecordKey: "value"
```

### Json Encoder

The json encoder takes each entry in the internal representation of the data and converts it to a json byte array.
These byte arrays may then be output by a `write` stage.


### Prometheus encoder

The prometheus encoder specifies which metrics to export to prometheus and which labels should be associated with those metrics.
For example, we may want to report the number of bytes and packets for the reported flows.
For each reported metric, we may specify a different set of labels.
Each metric may be renamed from its internal name.
The internal metric name is specified as `input` and the exported name is specified as `name`.
A prefix for all exported metrics may be specified, and this prefix is prepended to the `name` of each specified metric.

```yaml
pipeline:
  encode:
    type: prom
    prom:
      port: 9103
      prefix: test_
      metrics:
        - name: Bytes
          type: gauge
          valuekey: bytes
          labels:
            - srcAddr
            - dstAddr
            - srcPort
        - name: Packets
          type: counter
          valuekey: packets
          labels:
            - srcAddr
            - dstAddr
            - dstPort
```

In this example, for the `bytes` metric we report with the labels which specify srcAddr, dstAddr and srcPort.
Each different combination of label-values is a distinct gauge reported to prometheus.
The name of the prometheus gauge is set to `test_Bytes` by concatenating the prefix with the metric name.
The `packets` metric is very similar. It makes use of the `counter` prometheus type which adds reported values
to prometheus counter.

### Loki writer

The loki writer persist flow-logs into [Loki](https://github.com/grafana/loki). The flow-logs are sent with defined 
tenant ID and with a set static labels and dynamic labels from the record fields. 
For example, sending flow-logs into tenant `theTenant` with labels 
from `foo` and `bar` fields 
and including static label with key `job` with value `flowlogs2metrics`. 
Additional parameters such as `url` and `batchWait` are defined in 
Loki writer API [docs/api.md](docs/api.md)

```bash
pipeline:
  write:
    type: loki
    loki:
      tenantID: theTenant
      loki:
        url: http://loki.default.svc.cluster.local:3100
      staticLabels:
        job: flowlogs2metrics
      batchWait: 1m
      labels:
        - foo
        - bar
  ```

> Note: to view loki flow-logs in `grafana`:: Use the `Explore` tab and choose the `loki` datasource. In the `Log Browser` enter `{job="flowlogs2metrics"}` and press `Run query` 

# Development

## Build

- Clone this repository from github into a local machine (Linux/X86):
  `git clone git@github.com:netobserv/flowlogs2metrics.git`
- Change directory into flowlogs2metrics into:
  `cd flowlogs2metrics`
- Build the code:
  `make build`

FL2M uses `Makefile` to build, tests and deploy. Following is the output of `make help` :

<!---AUTO-makefile_help--->
```bash
  
Usage:  
  make <target>  
  
General  
  help                  Display this help.  
  
Develop  
  lint                  Lint the code  
  build                 Build flowlogs2metrics executable and update the docs  
  dashboards            Build grafana dashboards  
  docs                  Update flowlogs2metrics documentation  
  clean                 Clean  
  test                  Test  
  benchmarks            Benchmark  
  run                   Run  
  
Docker  
  push-image            Push latest image  
  
kubernetes  
  deploy                Deploy the image  
  undeploy              Undeploy the image  
  deploy-loki           Deploy loki  
  undeploy-loki         Undeploy loki  
  deploy-prometheus     Deploy prometheus  
  undeploy-prometheus   Undeploy prometheus  
  deploy-grafana        Deploy grafana  
  undeploy-grafana      Undeploy grafana  
  deploy-netflow-simulator  Deploy netflow simulator  
  undeploy-netflow-simulator  Undeploy netflow simulator  
  
kind  
  create-kind-cluster   Create cluster  
  delete-kind-cluster   Delete cluster  
  
metrics  
  generate-configuration  Generate metrics configuration  
  
End2End  
  local-deploy          Deploy locally on kind (with simulated flowlogs)  
  local-cleanup         Undeploy from local kind  
  local-redeploy        Redeploy locally (on current kind)  
  ocp-deploy            Deploy to OCP  
  ocp-cleanup           Undeploy from OCP  
  dev-local-deploy      Deploy locally with simulated netflows
```
<!---END-AUTO-makefile_help--->
