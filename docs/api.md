
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
                     agg_histogram: counts samples in configurable buckets, pre-aggregated via an Aggregate stage
                 filter: an optional criterion to filter entries by. Deprecated: use filters instead.
                     key: the key to match and filter by
                     value: the value to match and filter by. Use !nil / nil to match presence / absence. Add multiple matching values using '|' rune such as 'a|b' to match either 'a' or 'b'.
                 filters: a list of criteria to filter entries by
                         key: the key to match and filter by
                         value: the value to match and filter by. Use !nil / nil to match presence / absence. Add multiple matching values using '|' rune such as 'a|b' to match either 'a' or 'b'.
                 valueKey: entry key from which to resolve metric value
                 labels: labels to be associated with the metric
                 buckets: histogram buckets
         prefix: prefix added to each metric name
         expiryTime: time duration of no-flow to wait before deleting prometheus data item
         maxMetrics: maximum number of metrics to report (default: unlimited)
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
         tls: TLS client configuration (optional)
             insecureSkipVerify: skip client verifying the server's certificate chain and host name
             caCertPath: path to the CA certificate
             userCertPath: path to the user certificate
             userKeyPath: path to the user private key
         sasl: SASL configuration (optional)
             clientIDPath: path to the client ID / SASL username
             clientSecretPath: path to the client secret / SASL password
</pre>
## S3 encode API
Following is the supported API format for S3 encode:

<pre>
 s3:
         account: tenant id for this flow collector
         endpoint: address of s3 server
         accessKeyId: username to connect to server
         secretAccessKey: password to connect to server
         bucket: bucket into which to store objects
         writeTimeout: timeout (in seconds) for write operation
         batchSize: limit on how many flows will be buffered before being sent to an object
         secure: true for https, false for http (default: false)
         objectHeaderParameters: parameters to include in object header (key/value pairs)
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
         batchMaxLen: the number of accumulated flows before being forwarded for processing
         pullQueueCapacity: the capacity of the queue use to store pulled flows
         pullMaxBytes: the maximum number of bytes being pulled from kafka
         commitInterval: the interval (in milliseconds) at which offsets are committed to the broker.  If 0, commits will be handled synchronously.
         tls: TLS client configuration (optional)
             insecureSkipVerify: skip client verifying the server's certificate chain and host name
             caCertPath: path to the CA certificate
             userCertPath: path to the user certificate
             userKeyPath: path to the user private key
         sasl: SASL configuration (optional)
             clientIDPath: path to the client ID / SASL username
             clientSecretPath: path to the client secret / SASL password
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
                 multiplier: scaling factor to compenstate for sampling
</pre>
## Transform Filter API
Following is the supported API format for filter transformations:

<pre>
 filter:
         rules: list of filter rules, each includes:
                 input: entry input field
                 output: entry output field
                 type: (enum) one of the following:
                     remove_field: removes the field from the entry
                     remove_entry_if_exists: removes the entry if the field exists
                     remove_entry_if_doesnt_exist: removes the entry if the field does not exist
                     remove_entry_if_equal: removes the entry if the field value equals specified value
                     remove_entry_if_not_equal: removes the entry if the field value does not equal specified value
                     add_field_if_doesnt_exist: adds a field to the entry if the field does not exist
                     add_field_if: add output field set to assignee if input field satisfies criteria from parameters field
                     add_regex_if: add output field if input field satisfies regex pattern from parameters field
                 value: specified value of input field:
                 parameters: parameters specific to type
                 assignee: value needs to assign to output field
</pre>
## Transform Network API
Following is the supported API format for network transformations:

<pre>
 network:
         rules: list of transform rules, each includes:
                 input: entry input field
                 output: entry output field
                 type: (enum) one of the following:
                     add_subnet: add output subnet field from input field and prefix length from parameters field
                     add_location: add output location fields from input
                     add_service: add output network service field from input port and parameters protocol field
                     add_kubernetes: add output kubernetes fields from input
                     reinterpret_direction: reinterpret flow direction at the node level (instead of net interface), to ease the deduplication process
                     add_ip_category: categorize IPs based on known subnets configuration
                 parameters: parameters specific to type
                 assignee: value needs to assign to output field
         kubeConfigPath: path to kubeconfig file (optional)
         servicesFile: path to services file (optional, default: /etc/services)
         protocolsFile: path to protocols file (optional, default: /etc/protocols)
         ipCategories: configure IP categories
                 cidrs: list of CIDRs to match a category
                 name: name of the category
         directionInfo: information to reinterpret flow direction (optional, to use with reinterpret_direction rule)
             reporterIPField: field providing the reporter (agent) host IP
             srcHostField: source host field
             dstHostField: destination host field
             flowDirectionField: field providing the flow direction in the input entries; it will be rewritten
             ifDirectionField: interface-level field for flow direction, to create in output
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
         format: the format of each line: printf (default - writes using golang's default map printing), fields (writes one key and value field per line) or json
</pre>
## Aggregate metrics API
Following is the supported API format for specifying metrics aggregations:

<pre>
 aggregates:
         defaultExpiryTime: default time duration of data aggregation to perform rules (default: 2 minutes)
         rules: list of aggregation rules, each includes:
                 name: description of aggregation result
                 groupByKeys: list of fields on which to aggregate
                 operationType: sum, min, max, count, avg or raw_values
                 operationKey: internal field on which to perform the operation
                 expiryTime: time interval over which to perform the operation
</pre>
## Connection tracking API
Following is the supported API format for specifying connection tracking:

<pre>
 conntrack:
         keyDefinition: fields that are used to identify the connection
             fieldGroups: list of field group definitions
                     name: field group name
                     fields: list of fields in the group
             hash: how to build the connection hash
                 fieldGroupRefs: list of field group names to build the hash
                 fieldGroupARef: field group name of endpoint A
                 fieldGroupBRef: field group name of endpoint B
         outputRecordTypes: (enum) output record types to emit
             newConnection: New connection
             endConnection: End connection
             heartbeat: Heartbeat
             flowLog: Flow log
         outputFields: list of output fields
                 name: output field name
                 operation: (enum) aggregate operation on the field value
                     sum: sum
                     count: count
                     min: min
                     max: max
                     first: first
                     last: last
                 splitAB: When true, 2 output fields will be created. One for A->B and one for B->A flows.
                 input: The input field to base the operation on. When omitted, 'name' is used
                 reportMissing: When true, missing input will produce MissingFieldError metric and error logs
         scheduling: list of timeouts and intervals to apply per selector
                 selector: key-value map to match against connection fields to apply this scheduling
                 endConnectionTimeout: duration of time to wait from the last flow log to end a connection
                 terminatingTimeout: duration of time to wait from detected FIN flag to end a connection
                 heartbeatInterval: duration of time to wait between heartbeat reports of a connection
         maxConnectionsTracked: maximum number of connections we keep in our cache (0 means no limit)
         tcpFlags: settings for handling TCP flags
             fieldName: name of the field containing TCP flags
             detectEndConnection: detect end connections by FIN flag
             swapAB: swap source and destination when the first flowlog contains the SYN_ACK flag
</pre>
## Time-based Filters API
Following is the supported API format for specifying metrics time-based filters:

<pre>
 timebased:
         rules: list of filter rules, each includes:
                 name: description of filter result
                 indexKey: internal field to index TopK. Deprecated, use indexKeys instead
                 indexKeys: internal fields to index TopK
                 operationType: (enum) sum, min, max, avg, count, last or diff
                     sum: set output field to sum of parameters fields in the time window
                     avg: set output field to average of parameters fields in the time window
                     min: set output field to minimum of parameters fields in the time window
                     max: set output field to maximum of parameters fields in the time window
                     count: set output field to number of flows registered in the time window
                     last: set output field to last of parameters fields in the time window
                     diff: set output field to the difference of the first and last parameters fields in the time window
                 operationKey: internal field on which to perform the operation
                 topK: number of highest incidence to report (default - report all)
                 reversed: report lowest incidence instead of highest (default - false)
                 timeInterval: time duration of data to use to compute the metric
</pre>