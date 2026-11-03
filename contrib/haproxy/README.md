# OpenShift HAProxy request records

This input receives the versioned HTTP access log in [log-format.txt](log-format.txt)
using UDP syslog. It relates backend TCP tuples to the client reported by HAProxy,
without uprobes. HTTP, edge and reencrypt Routes are supported. TLS passthrough,
Envoy, user-defined networks and original identities inferred from HTTP headers
are outside this first implementation.

A record describes one completed HTTP request. Backend connection reuse can
associate several clients with the same TCP connection. Records never supply
flow byte/packet counts, and must bypass conntrack, deduplication and flow metrics.
The operator provisions a separate pipeline, Kubernetes enrichment and a Loki
stream with `app="netobserv-proxy"` and `_RecordType="proxyRequest"`.

## Enable with the operator

Configure a source on the FlowCollector:

```yaml
spec:
  processor:
    proxyCorrelation:
      enable: true
      sources:
        - name: openshift-router
          type: HAProxy
          haproxy:
            port: 10514
```

Loki is required. The optional `port` defaults to UDP 10514. The receiver is exposed by
`flowlogs-pipeline-proxy` in Service/Direct deployments, and by
`flowlogs-pipeline-transformer-proxy` with Kafka. Kafka deployments receive these
logs directly in the processors; proxy records bypass Kafka. UDP delivery is best
effort and is not durable.

Read that Service's allocated `spec.clusterIP` and configure the intended
IngressController explicitly:

```yaml
spec:
  logging:
    access:
      destination:
        type: Syslog
        syslog:
          address: <receiver Service clusterIP>
          port: 10514
          facility: local0
          maxLength: 2048
      httpLogFormat: <exact single line from log-format.txt>
```

The operator does not change IngressControllers. Review existing logging first:
changing the destination replaces it; use a relay if you need to retain another
consumer. The destination requires an IP address, and must be updated if the
Service is recreated with a new address. Ensure router egress policies allow it.
The operator's ingress NetworkPolicy allows this port from `openshift-ingress`
and the OVN host-network peer where applicable. Host-network rules cannot
authenticate individual routers. UDP syslog carries no sender authentication;
keep this receiver private and control access to its network.

If an external LB hides the client source address, it must provide that identity
to the router (for example, through an explicitly configured PROXY protocol).
Without it, the record identifies the LB. The receiver does not trust X-Forwarded-For.

## Timing and limits

`%trg` supplies UTC request start time to the second, `%tr` supplies request-start
milliseconds, and `%Ta` supplies active request duration in milliseconds. These
are request timestamps, not the connection accept timestamp. Partial/early logs
(with `+` duration), missing backend sockets, unknown versions, invalid endpoints
and durations beyond 24 hours are discarded. Do not enable `logasap` for this
contract. Health checks without a backend do not produce correlation records.
`ProxyRequestID` scopes HAProxy's request counter to router/process/start time.

Loki indexes reception time (`TimeReceived`) to accommodate end-of-request logs.
The console matches the full tuple in either direction and overlapping request
intervals with a two-second clock tolerance. It searches reception time from the
flow start minus two seconds through flow end plus ten minutes, capped at now.
A lookup accepts flow intervals up to 24 hours and returns at most 1000 requests.
It reports its coverage, pending window and result limit. Long-running requests,
buffering, packet loss or missing configuration can leave gaps. An empty result
is not proof that no clients used the connection.

The first console integration is administrator-only. Multi-cluster lookups require
the flow's cluster identity; overlapping network addresses without identity and
user-defined networks are unsupported. Request URLs, headers and bodies are not
collected. IP/port/request IDs are not Loki stream labels.

## Standalone FLP

A minimal receiver can be configured as follows (add enrichment and a writer):

```yaml
pipeline:
  - name: proxy
parameters:
  - name: proxy
    ingest:
      type: haproxy
      haproxy:
        address: ':10514'
        bufferLength: 1000
        maxMessageSize: 2048
```

The queue and datagram size are bounded. Oversized, unsupported, incomplete,
malformed, backend-less and queue-full messages increment the existing ingestion
error counter with a bounded error category, without logging the original payload.
The socket is closed when the pipeline shuts down.
