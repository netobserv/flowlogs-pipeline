
> Note: this file was automatically generated, to update execute "make docs"  
	 
# flowlogs-pipeline Operational Metrics  
	 
Each table below provides documentation for an exported flowlogs-pipeline operational metric. 

	

### conntrack_input_records
| **Name** | conntrack_input_records | 
|:---|:---|
| **Description** | The total number of input records per classification. | 
| **Type** | counter | 
| **Labels** | classification | 


### conntrack_memory_connections
| **Name** | conntrack_memory_connections | 
|:---|:---|
| **Description** | The total number of tracked connections in memory. | 
| **Type** | gauge | 
| **Labels** |  | 


### conntrack_output_records
| **Name** | conntrack_output_records | 
|:---|:---|
| **Description** | The total number of output records. | 
| **Type** | counter | 
| **Labels** | type | 


### encode_prom_errors
| **Name** | encode_prom_errors | 
|:---|:---|
| **Description** | Total errors during metrics generation | 
| **Type** | counter | 
| **Labels** | error, metric, key | 


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
| **Description** | Provides number of flows processed, batches processed, and batch size stats (in number of flows) | 
| **Type** | summary | 
| **Labels** | stage | 


### ingest_latency_ms
| **Name** | ingest_latency_ms | 
|:---|:---|
| **Description** | Latency between flow end time and ingest time, in milliseconds | 
| **Type** | histogram | 
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


