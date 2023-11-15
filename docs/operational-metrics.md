
> Note: this file was automatically generated, to update execute "make docs"  
	 
# flowlogs-pipeline Operational Metrics  
	 
Each table below provides documentation for an exported flowlogs-pipeline operational metric. 

	

### conntrack_aggregator_errors
| **Name** | conntrack_aggregator_errors | 
|:---|:---|
| **Description** | The total number of errors during aggregation | 
| **Type** | counter | 
| **Labels** | error, field | 


### conntrack_end_connections
| **Name** | conntrack_end_connections | 
|:---|:---|
| **Description** | The total number of connections ended per group and reason | 
| **Type** | counter | 
| **Labels** | group, reason | 


### conntrack_hash_errors
| **Name** | conntrack_hash_errors | 
|:---|:---|
| **Description** | The total number of errors during hash computation | 
| **Type** | counter | 
| **Labels** | error, field | 


### conntrack_input_records
| **Name** | conntrack_input_records | 
|:---|:---|
| **Description** | The total number of input records per classification | 
| **Type** | counter | 
| **Labels** | classification | 


### conntrack_memory_connections
| **Name** | conntrack_memory_connections | 
|:---|:---|
| **Description** | The total number of tracked connections in memory per group and phase | 
| **Type** | gauge | 
| **Labels** | group, phase | 


### conntrack_output_records
| **Name** | conntrack_output_records | 
|:---|:---|
| **Description** | The total number of output records | 
| **Type** | counter | 
| **Labels** | type | 


### conntrack_tcp_flags
| **Name** | conntrack_tcp_flags | 
|:---|:---|
| **Description** | The total number of actions taken based on TCP flags | 
| **Type** | counter | 
| **Labels** | action | 


### encode_prom_errors
| **Name** | encode_prom_errors | 
|:---|:---|
| **Description** | Total errors during metrics generation | 
| **Type** | counter | 
| **Labels** | error, metric, key | 


### encode_prom_metrics_reported
| **Name** | encode_prom_metrics_reported | 
|:---|:---|
| **Description** | Total number of prometheus metrics reported by this stage | 
| **Type** | gauge | 
| **Labels** | stage | 


### ingest_batch_size_bytes
| **Name** | ingest_batch_size_bytes | 
|:---|:---|
| **Description** | Ingested batch size distribution, in bytes | 
| **Type** | summary | 
| **Labels** | stage | 


### ingest_errors
| **Name** | ingest_errors | 
|:---|:---|
| **Description** | Counter of errors during ingestion | 
| **Type** | counter | 
| **Labels** | stage, type, code | 


### ingest_flows_processed
| **Name** | ingest_flows_processed | 
|:---|:---|
| **Description** | Number of flows received by the ingester | 
| **Type** | counter | 
| **Labels** | stage | 


### ingest_latency_ms
| **Name** | ingest_latency_ms | 
|:---|:---|
| **Description** | Latency between flow end time and ingest time, in milliseconds | 
| **Type** | histogram | 
| **Labels** | stage | 


### ingest_synthetic_flows_processed
| **Name** | ingest_synthetic_flows_processed | 
|:---|:---|
| **Description** | Number of flow logs processed | 
| **Type** | counter | 
| **Labels** | stage | 


### metrics_dropped
| **Name** | metrics_dropped | 
|:---|:---|
| **Description** | Number of metrics dropped | 
| **Type** | counter | 
| **Labels** | stage | 


### metrics_dropped
| **Name** | metrics_dropped | 
|:---|:---|
| **Description** | Number of metrics dropped | 
| **Type** | counter | 
| **Labels** | stage | 


### metrics_processed
| **Name** | metrics_processed | 
|:---|:---|
| **Description** | Number of metrics processed | 
| **Type** | counter | 
| **Labels** | stage | 


### metrics_processed
| **Name** | metrics_processed | 
|:---|:---|
| **Description** | Number of metrics processed | 
| **Type** | counter | 
| **Labels** | stage | 


### records_written
| **Name** | records_written | 
|:---|:---|
| **Description** | Number of output records written | 
| **Type** | counter | 
| **Labels** | stage | 


### stage_duration_ms
| **Name** | stage_duration_ms | 
|:---|:---|
| **Description** | Pipeline stage duration in milliseconds | 
| **Type** | histogram | 
| **Labels** | stage | 


### stage_in_queue_size
| **Name** | stage_in_queue_size | 
|:---|:---|
| **Description** | Pipeline stage input queue size (number of elements in queue) | 
| **Type** | gauge | 
| **Labels** | stage | 


### stage_out_queue_size
| **Name** | stage_out_queue_size | 
|:---|:---|
| **Description** | Pipeline stage output queue size (number of elements in queue) | 
| **Type** | gauge | 
| **Labels** | stage | 


