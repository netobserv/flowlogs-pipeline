
## Prometheus encode API
Following is the supported API format for prometheus encode:

<pre>
 prom:
         metrics: list of prometheus metric definitions, each includes:
                 name: the metric name
                 type: (enum) one of the following:
                     gauge: single numerical value that can arbitrarily go up and down
                     counter: monotonically increasing counter whose value can only increase
                     histogram: counts samples in configurable buckets
                 filter: the criterion to filter entries by
                     key: the key to match and filter by
                     value: the value to match and filter by
                 valueKey: entry key from which to resolve metric value
                 labels: labels to be associated with the metric
                 buckets: histogram buckets
         port: port number to expose "/metrics" endpoint
         prefix: prefix added to each metric name
         expiryTime: seconds of no-flow to wait before deleting prometheus data item
</pre>
## Kafka encode API
Following is the supported API format for kafka encode:

<pre>
 kafka:
         address: address of kafka server
         topic: kafka topic to write to
         balancer: (enum) one of the following:
             roundRobin: RoundRobin balancer
             leastBytes: LeastBytes balancer
             hash: Hash balancer
             crc32: Crc32 balancer
             murmur2: Murmur2 balancer
         writeTimeout: timeout (in seconds) for write operation performed by the Writer
         readTimeout: timeout (in seconds) for read operation performed by the Writer
         batchBytes: limit the maximum size of a request in bytes before being sent to a partition
         batchSize: limit on how many messages will be buffered before being sent to a partition
</pre>
## Ingest collector API
Following is the supported API format for the NetFlow / IPFIX collector:

<pre>
 collector:
         hostName: the hostname to listen on
         port: the port number to listen on, for IPFIX/NetFlow v9. Omit or set to 0 to disable IPFIX/NetFlow v9 ingestion
         portLegacy: the port number to listen on, for legacy NetFlow v5. Omit or set to 0 to disable NetFlow v5 ingestion
         batchMaxLen: the number of accumulated flows before being forwarded for processing
</pre>
## Ingest Kafka API
Following is the supported API format for the kafka ingest:

<pre>
 kafka:
         brokers: list of kafka broker addresses
         topic: kafka topic to listen on
         groupid: separate groupid for each consumer on specified topic
         groupBalancers: list of balancing strategies (range, roundRobin, rackAffinity)
         startOffset: FirstOffset (least recent - default) or LastOffset (most recent) offset available for a partition
         batchReadTimeout: how often (in milliseconds) to process input
         decoder: decoder to use (E.g. json or protobuf)
             type: (enum) one of the following:
                 json: JSON decoder
                 protobuf: Protobuf decoder
</pre>
## Ingest GRPC from Network Observability eBPF Agent
Following is the supported API format for the Network Observability eBPF ingest:

<pre>
 grpc:
         port: the port number to listen on
         bufferLength: the length of the ingest channel buffer, in groups of flows, containing each group hundreds of flows (default: 100)
</pre>
## Transform Generic API
Following is the supported API format for generic transformations:

<pre>
 generic:
         policy: (enum) key replacement policy; may be one of the following:
             preserve_original_keys: adds new keys in addition to existing keys (default)
             replace_keys: removes all old keys and uses only the new keys
         rules: list of transform rules, each includes:
                 input: entry input field
                 output: entry output field
</pre>
## Transform Filter API
Following is the supported API format for filter transformations:

<pre>
 filter:
         rules: list of filter rules, each includes:
                 input: entry input field
                 type: (enum) one of the following:
                     remove_field: removes the field from the entry
                     remove_entry_if_exists: removes the entry if the field exists
                     remove_entry_if_doesnt_exist: removes the entry if the field doesnt exist
</pre>
## Transform Network API
Following is the supported API format for network transformations:

<pre>
 network:
         rules: list of transform rules, each includes:
                 input: entry input field
                 output: entry output field
                 type: (enum) one of the following:
                     conn_tracking: set output field to value of parameters field only for new flows by matching template in input field
                     add_regex_if: add output field if input field satisfies regex pattern from parameters field
                     add_if: add output field if input field satisfies criteria from parameters field
                     add_subnet: add output subnet field from input field and prefix length from parameters field
                     add_location: add output location fields from input
                     add_service: add output network service field from input port and parameters protocol field
                     add_kubernetes: add output kubernetes fields from input
                 parameters: parameters specific to type
         kubeConfigPath: path to kubeconfig file (optional)
         servicesFile: path to services file (optional, default: /etc/services)
         protocolsFile: path to protocols file (optional, default: /etc/protocols)
</pre>
## Write Loki API
Following is the supported API format for writing to loki:

<pre>
 loki:
         url: the address of an existing Loki service to push the flows to
         tenantID: identifies the tenant for the request
         batchWait: maximum amount of time to wait before sending a batch
         batchSize: maximum batch size (in bytes) of logs to accumulate before sending
         timeout: maximum time to wait for a server to respond to a request
         minBackoff: initial backoff time for client connection between retries
         maxBackoff: maximum backoff time for client connection between retries
         maxRetries: maximum number of retries for client connections
         labels: map of record fields to be used as labels
         staticLabels: map of common labels to set on each flow
         ignoreList: map of record fields to be removed from the record
         clientConfig: clientConfig
         timestampLabel: label to use for time indexing
         timestampScale: timestamp units scale (e.g. for UNIX = 1s)
</pre>
## Write Standard Output
Following is the supported API format for writing to standard output:

<pre>
 stdout:
         format: the format of each line: printf (default), fields or json
</pre>
## Aggregate metrics API
Following is the supported API format for specifying metrics aggregations:

<pre>
 aggregates:
         name: description of aggregation result
         by: list of fields on which to aggregate
         operation: sum, min, max, avg or raw_values
         recordKey: internal field on which to perform the operation
         topK: number of highest incidence to report (default - report all)
</pre>