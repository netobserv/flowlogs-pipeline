
> Note: this file was automatically generated, to update execute "make generate-configuration"  
> Note: the data was generated from network definitions under the network_definitions folder  
  
# flowlogs2metrics Metrics  
  
Each table below provides documentation for an exported flowlogs2metrics metric. 
The documentation describes the metric, the collected information from network flow-logs
and the transformation to generate the exported metric.
  
  

	

### bandwidth per network service
| **Description** | This metric observes the network bandwidth per network service | 
|:---|:---|
| **Details** | Sum bytes for all traffic per network service | 
| **Usage** | Evaluate network usage breakdown per network service | 
| **Labels** | bandwidth, graph, rate, network-service |
| **Operation** | aggregate by `service` and `sum` field `bytes` |
| **Exposed as** | `fl2m_bandwidth_per_network_service` of type `gauge` |
| **Visualized as** | "Bandwidth per network service" on dashboard `details` |
|||  


### bandwidth per src dest subnet
| **Description** | This metric observes the network bandwidth per source and destination subnets | 
|:---|:---|
| **Details** | Sum bandwidth bytes for all traffic per source / destination subnet pair | 
| **Usage** | Evaluate network usage breakdown per source / destination subnet pair | 
| **Labels** | bandwidth, graph, rate, subnet |
| **Operation** | aggregate by `dstSubnet24, srcSubnet24` and `sum` field `bytes` |
| **Exposed as** | `fl2m_bandwidth_per_source_destination_subnet` of type `gauge` |
| **Visualized as** | "Bandwidth per src and destination subnet" on dashboard `details` |
|||  


### bandwidth per src subnet
| **Description** | This metric observes the network bandwidth per source subnet | 
|:---|:---|
| **Details** | Sum bytes for all traffic per source subnet | 
| **Usage** | Evaluate network usage breakdown per source subnet | 
| **Labels** | bandwidth, graph, rate, subnet |
| **Operation** | aggregate by `srcSubnet` and `sum` field `bytes` |
| **Exposed as** | `fl2m_bandwidth_per_source_subnet` of type `gauge` |
| **Visualized as** | "Bandwidth per source subnet" on dashboard `details` |
|||  


### connection rate per dest subnet
| **Description** | This metric observes network connections rate per destination subnet | 
|:---|:---|
| **Details** | Counts the number of connections per subnet with network prefix length /16 (using conn_tracking sum isNewFlow field) | 
| **Usage** | Evaluate network connections per subnet | 
| **Labels** | rate, subnet |
| **Operation** | aggregate by `dstSubnet` and `sum`  |
| **Exposed as** | `fl2m_connections_per_destination_subnet` of type `gauge` |
| **Visualized as** | "Connections rate per destinationIP /16 subnets" on dashboard `details` |
|||  


### connection rate per src subnet
| **Description** | This metric observes network connections rate per source subnet | 
|:---|:---|
| **Details** | Counts the number of connections per subnet with network prefix length /16 | 
| **Usage** | Evaluate network connections per subnet | 
| **Labels** | rate, subnet |
| **Operation** | aggregate by `srcSubnet` and `count`  |
| **Exposed as** | `fl2m_connections_per_source_subnet` of type `gauge` |
| **Visualized as** | "Connections rate per sourceIP /16 subnets" on dashboard `details` |
|||  


### connection rate per tcp flags
| **Description** | This metric observes network connections rate per TCPFlags | 
|:---|:---|
| **Details** | Counts the number of connections per tcp flags | 
| **Usage** | Evaluate difference in connections rate of different TCP Flags. Can be used, for example, to identify syn-attacks. | 
| **Labels** | rate, TCPFlags |
| **Operation** | aggregate by `TCPFlags` and `count`  |
| **Exposed as** | `fl2m_connections_per_tcp_flags` of type `gauge` |
| **Visualized as** | "Connections rate per TCPFlags" on dashboard `details` |
|||  


### connections per dst as
| **Description** | This metric counts network connections per destination Autonomous System (AS) | 
|:---|:---|
| **Details** | Aggregates flow records by values of "DstAS" field and counts the number of entries in each aggregate with non zero value | 
| **Usage** | Evaluate amount of connections targeted at different Autonomous Systems | 
| **Labels** | rate, count, AS |
| **Operation** | aggregate by `dstAS` and `count`  |
| **Exposed as** | `fl2m_connections_per_destination_as` of type `gauge` |
| **Visualized as** | "Connections rate per destination AS" on dashboard `details` |
|||  


### connections per src as
| **Description** | This metric counts network connections per source Autonomous System (AS) | 
|:---|:---|
| **Details** | Aggregates flow records by values of "SrcAS" field and counts the number of entries in each aggregate with non zero value | 
| **Usage** | Evaluate amount of connections initiated by different Autonomous Systems | 
| **Labels** | rate, count, AS |
| **Operation** | aggregate by `srcAS` and `count`  |
| **Exposed as** | `fl2m_connections_per_source_as` of type `gauge` |
| **Visualized as** | "Connections rate per source AS" on dashboard `details` |
|||  


### count per src dest subnet
| **Description** | This metric counts the number of distinct source / destination subnet pairs | 
|:---|:---|
| **Details** | Count the number of distinct source / destination subnet pairs | 
| **Usage** | Evaluate network usage breakdown per source / destination subnet pair | 
| **Labels** | count, graph, rate, subnet |
| **Operation** | aggregate by `dstSubnet24, srcSubnet24` and `count`  |
| **Exposed as** | `fl2m_count_per_source_destination_subnet` of type `gauge` |
| **Visualized as** | "Connections rate of src / destination subnet occurences" on dashboard `details` |
|||  


### egress bandwidth per dest subnet
| **Description** | This metric observes the network bandwidth per destination subnet | 
|:---|:---|
| **Details** | Sum egress bytes for all traffic per destination subnet | 
| **Usage** | Evaluate network usage breakdown per destination subnet | 
| **Labels** | bandwidth, graph, rate, subnet |
| **Operation** | aggregate by `dstSubnet` and `sum` field `bytes` |
| **Exposed as** | `fl2m_egress_per_destination_subnet` of type `gauge` |
| **Visualized as** | "Bandwidth per destination subnet" on dashboard `details` |
| **Visualized as** | "Total bandwidth" on dashboard `totals` |
|||  


### egress bandwidth per namespace
| **Description** | This metric observes the network bandwidth per namespace | 
|:---|:---|
| **Details** | Sum egress bytes for all traffic per namespace | 
| **Usage** | Evaluate network usage breakdown per namespace | 
| **Labels** | kubernetes, bandwidth, graph |
| **Operation** | aggregate by `srcK8S_Namespace, srcK8S_Type` and `sum` field `bytes` |
| **Exposed as** | `fl2m_egress_per_namespace` of type `gauge` |
| **Visualized as** | "Bandwidth per namespace" on dashboard `details` |
|||  


### geo location rate per dest
| **Description** | This metric observes connections geo-location rate per destination IP | 
|:---|:---|
| **Details** | Counts the number of connections per geo-location based on destination IP | 
| **Usage** | Evaluate network connections geo-location | 
| **Labels** | rate, connections-count, geo-location, destinationIP |
| **Operation** | aggregate by `dstLocation_CountryName` and `count`  |
| **Exposed as** | `fl2m_connections_per_destination_location` of type `gauge` |
| **Visualized as** | "Connections rate per destinationIP geo-location" on dashboard `details` |
|||  


### mice elephants
| **Description** | This metric counts mice and elephant flows | 
|:---|:---|
| **Details** | Count connections with bytes lower than 0.5K and bigger than 0.5K | 
| **Usage** | Evaluate network behaviour | 
| **Labels** | bandwidth, mice, elephant, rate |
| **Operation** | aggregate by `mice_Evaluate` and `count`  |
| **Operation** | aggregate by `elephant_Evaluate` and `count`  |
| **Exposed as** | `fl2m_mice_count` of type `gauge` |
| **Exposed as** | `fl2m_elephant_count` of type `gauge` |
| **Visualized as** | "Mice flows count" on dashboard `details` |
| **Visualized as** | "Elephant flows count" on dashboard `details` |
|||  


### network services count
| **Description** | This metric observes network services rate (total) | 
|:---|:---|
| **Details** | Counts the number of connections per network service based on destination port number and protocol | 
| **Usage** | Evaluate network services | 
| **Labels** | rate, network-services, destination-port, destination-protocol |
| **Operation** | aggregate by `service` and `count`  |
| **Exposed as** | `fl2m_service_count` of type `gauge` |
| **Visualized as** | "Network services connections rate" on dashboard `details` |
| **Visualized as** | "Number of network services" on dashboard `totals` |
|||  


