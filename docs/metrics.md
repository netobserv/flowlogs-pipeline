
> Note: this file was automatically generated, to update execute "make generate-configuration"  
> Note: the data was generated from network definitions under the network_definitions folder  
  
# flowlogs-pipeline Metrics  
  
Each table below provides documentation for an exported flowlogs-pipeline metric. 
The documentation describes the metric, the collected information from network flow-logs
and the transformation to generate the exported metric.
  
  

	

### bandwidth per network service
| **Description** | This metric observes the network bandwidth per network service | 
|:---|:---|
| **Details** | Sum bytes for all traffic per network service | 
| **Usage** | Evaluate network usage breakdown per network service | 
| **Tags** | bandwidth, graph, rate, network-service |
| **Operation** | aggregate by `service, _RecordType` and `sum` field `bytes` |
| **Exposed as** | `flp_bandwidth_per_network_service` of type `counter` |
| **Visualized as** | "Bandwidth per network service" on dashboard `details` |
|||  


### bandwidth per src dest subnet
| **Description** | This metric observes the network bandwidth per source and destination subnets | 
|:---|:---|
| **Details** | Sum bandwidth bytes for all traffic per source / destination subnet pair | 
| **Usage** | Evaluate network usage breakdown per source / destination subnet pair | 
| **Tags** | bandwidth, graph, rate, subnet |
| **Operation** | aggregate by `dstSubnet24, srcSubnet24, _RecordType` and `sum` field `bytes` |
| **Exposed as** | `flp_bandwidth_per_source_destination_subnet` of type `counter` |
| **Visualized as** | "Bandwidth per src and destination subnet" on dashboard `details` |
|||  


### bandwidth per src subnet
| **Description** | This metric observes the network bandwidth per source subnet | 
|:---|:---|
| **Details** | Sum bytes for all traffic per source subnet | 
| **Usage** | Evaluate network usage breakdown per source subnet | 
| **Tags** | bandwidth, graph, rate, subnet |
| **Operation** | aggregate by `srcSubnet, _RecordType` and `sum` field `bytes` |
| **Exposed as** | `flp_bandwidth_per_source_subnet` of type `counter` |
| **Visualized as** | "Bandwidth per source subnet" on dashboard `details` |
|||  


### connection length histogram
| **Description** | A histogram of connection size in bytes | 
|:---|:---|
| **Details** | Connection size in bytes distribution over time | 
| **Usage** | Evaluate connection size behavior including mice/elephant use-case | 
| **Tags** | bandwidth, mice, elephant, rate |
| **Operation** | aggregate by `_RecordType` and `raw_values` field `bytes_total` |
| **Operation** | aggregate by `_RecordType` and `raw_values` field `bytes_AB` |
| **Operation** | aggregate by `_RecordType` and `raw_values` field `bytes_BA` |
| **Exposed as** | `flp_connection_size_histogram` of type `histogram` |
| **Exposed as** | `flp_connection_size_histogram_ab` of type `histogram` |
| **Exposed as** | `flp_connection_size_histogram_ba` of type `histogram` |
| **Visualized as** | "Connection size in bytes heatmap" on dashboard `details` |
| **Visualized as** | "Connection size in bytes histogram" on dashboard `totals` |
|||  


### connection rate per dest subnet
| **Description** | This metric observes network connections rate per destination subnet | 
|:---|:---|
| **Details** | Counts the number of connections per subnet with network prefix length /16 (using conn_tracking sum isNewFlow field) | 
| **Usage** | Evaluate network connections per subnet | 
| **Tags** | rate, subnet |
| **Operation** | aggregate by `dstSubnet, _RecordType` and `count` field `isNewFlow` |
| **Exposed as** | `flp_connections_per_destination_subnet` of type `counter` |
| **Visualized as** | "Connections rate per destinationIP /16 subnets" on dashboard `details` |
|||  


### connection rate per src subnet
| **Description** | This metric observes network connections rate per source subnet | 
|:---|:---|
| **Details** | Counts the number of connections per subnet with network prefix length /16 | 
| **Usage** | Evaluate network connections per subnet | 
| **Tags** | rate, subnet |
| **Operation** | aggregate by `srcSubnet, _RecordType` and `count`  |
| **Exposed as** | `flp_connections_per_source_subnet` of type `counter` |
| **Visualized as** | "Connections rate per sourceIP /16 subnets" on dashboard `details` |
|||  


### connection rate per tcp flags
| **Description** | This metric observes network connections rate per TCPFlags | 
|:---|:---|
| **Details** | Counts the number of connections per tcp flags | 
| **Usage** | Evaluate difference in connections rate of different TCP Flags. Can be used, for example, to identify syn-attacks. | 
| **Tags** | rate, TCPFlags |
| **Operation** | aggregate by `TCPFlags, _RecordType` and `count`  |
| **Exposed as** | `flp_connections_per_tcp_flags` of type `counter` |
| **Visualized as** | "Connections rate per TCPFlags" on dashboard `details` |
|||  


### connections per dst as
| **Description** | This metric counts network connections per destination Autonomous System (AS) | 
|:---|:---|
| **Details** | Aggregates flow records by values of "DstAS" field and counts the number of entries in each aggregate with non zero value | 
| **Usage** | Evaluate amount of connections targeted at different Autonomous Systems | 
| **Tags** | rate, count, AS |
| **Operation** | aggregate by `dstAS, _RecordType` and `count`  |
| **Exposed as** | `flp_connections_per_destination_as` of type `counter` |
| **Visualized as** | "Connections rate per destination AS" on dashboard `details` |
|||  


### connections per src as
| **Description** | This metric counts network connections per source Autonomous System (AS) | 
|:---|:---|
| **Details** | Aggregates flow records by values of "SrcAS" field and counts the number of entries in each aggregate with non zero value | 
| **Usage** | Evaluate amount of connections initiated by different Autonomous Systems | 
| **Tags** | rate, count, AS |
| **Operation** | aggregate by `srcAS, _RecordType` and `count`  |
| **Exposed as** | `flp_connections_per_source_as` of type `counter` |
| **Visualized as** | "Connections rate per source AS" on dashboard `details` |
|||  


### count per src dest subnet
| **Description** | This metric counts the number of distinct source / destination subnet pairs | 
|:---|:---|
| **Details** | Count the number of distinct source / destination subnet pairs | 
| **Usage** | Evaluate network usage breakdown per source / destination subnet pair | 
| **Tags** | count, graph, rate, subnet |
| **Operation** | aggregate by `dstSubnet24, srcSubnet24, _RecordType` and `count`  |
| **Exposed as** | `flp_count_per_source_destination_subnet` of type `counter` |
| **Visualized as** | "Connections rate of src / destination subnet occurences" on dashboard `details` |
|||  


### egress bandwidth per dest subnet
| **Description** | This metric observes the network bandwidth per destination subnet | 
|:---|:---|
| **Details** | Sum egress bytes for all traffic per destination subnet | 
| **Usage** | Evaluate network usage breakdown per destination subnet | 
| **Tags** | bandwidth, graph, rate, subnet |
| **Operation** | aggregate by `dstSubnet, _RecordType` and `sum` field `bytes` |
| **Exposed as** | `flp_egress_per_destination_subnet` of type `counter` |
| **Visualized as** | "Bandwidth per destination subnet" on dashboard `details` |
| **Visualized as** | "Total bandwidth" on dashboard `totals` |
|||  


### egress bandwidth per namespace
| **Description** | This metric observes the network bandwidth per namespace | 
|:---|:---|
| **Details** | Sum egress bytes for all traffic per namespace | 
| **Usage** | Evaluate network usage breakdown per namespace | 
| **Tags** | kubernetes, bandwidth, graph |
| **Operation** | aggregate by `srcK8S_Namespace, srcK8S_Type, _RecordType` and `sum` field `bytes` |
| **Exposed as** | `flp_egress_per_namespace` of type `counter` |
| **Visualized as** | "Bandwidth per namespace" on dashboard `details` |
|||  


### flows length histogram
| **Description** | A histogram of flowlog bytes | 
|:---|:---|
| **Details** | Flows length distribution over time | 
| **Usage** | Evaluate flows length behavior including mice/elephant use-case | 
| **Tags** | bandwidth, mice, elephant, rate |
| **Operation** | aggregate by `all_Evaluate, _RecordType` and `raw_values` field `bytes` |
| **Exposed as** | `flp_flows_length_histogram` of type `histogram` |
| **Visualized as** | "Flows length heatmap" on dashboard `details` |
| **Visualized as** | "Flows length histogram" on dashboard `totals` |
|||  


### geo location rate per dest
| **Description** | This metric observes connections geo-location rate per destination IP | 
|:---|:---|
| **Details** | Counts the number of connections per geo-location based on destination IP | 
| **Usage** | Evaluate network connections geo-location | 
| **Tags** | rate, connections-count, geo-location, destinationIP |
| **Operation** | aggregate by `dstLocation_CountryName, _RecordType` and `count`  |
| **Exposed as** | `flp_connections_per_destination_location` of type `counter` |
| **Visualized as** | "Connections rate per destinationIP geo-location" on dashboard `details` |
|||  


### loki bandwidth per namespace
| **Description** | This metric observes the bandwidth per namespace (from Loki) | 
|:---|:---|
| **Details** | Sum bytes for all traffic per source namespace | 
| **Usage** | Evaluate network usage breakdown per source namespace | 
| **Tags** | loki, graph, rate, namespace |
| **Visualized as** | "Bandwidth per source namespace" on dashboard `details` |
|||  


### loki logs per sec
| **Description** | This metric observes the number of loki logs | 
|:---|:---|
| **Details** | Rate of loki logs per sec | 
| **Usage** | Evaluate loki service usage | 
| **Tags** | loki, graph, rate |
| **Visualized as** | "Loki logs rate" on dashboard `details` |
|||  


### network services count
| **Description** | This metric observes network services rate (total) | 
|:---|:---|
| **Details** | Counts the number of connections per network service based on destination port number and protocol | 
| **Usage** | Evaluate network services | 
| **Tags** | rate, network-services, destination-port, destination-protocol |
| **Operation** | aggregate by `service, _RecordType` and `count`  |
| **Exposed as** | `flp_service_count` of type `counter` |
| **Visualized as** | "Network services connections rate" on dashboard `details` |
| **Visualized as** | "Number of network services" on dashboard `totals` |
|||  


