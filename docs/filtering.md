# FLP filtering language

Flowlogs-pipeline uses a simple query language to filter network flows:

```
(srcnamespace="netobserv" OR (srcnamespace="ingress" AND dstnamespace="netobserv")) AND srckind!="service"
```

The syntax includes:

- Logical boolean operators (case insensitive)
  - `and`
  - `or`
- Comparison operators
  - equals `=`
  - not equals `!=`
  - matches regexp `=~`
  - not matches regexp `!~`
  - greater than (or equal) `>` / `>=`
  - less than (or equal) `<` / `<=`
- Unary operations
  - field is present: `with(field)`
  - field is absent: `without(field)`
- Parenthesis-based priority

## API integration

The language is currently integrated in the "keep_entry" transform/filtering API. Example:

```yaml
    transform:
      type: filter
      filter:
        rules:
        - type: keep_entry_query
          keepEntryQuery: (namespace="A" and with(workload)) or service=~"abc.+"
          keepEntrySampling: 10 # Optionally, a sampling interval can be associated with the filter
```

## Integration with the NetObserv operator

In the [NetObserv operator](https://github.com/netobserv/network-observability-operator), the filtering query language is used in `FlowCollector` `spec.processor.filters`. Example:

```yaml
spec:
  processor:
    filters:
      - query: |
          (SrcK8S_Namespace="netobserv" OR (SrcK8S_Namespace="openshift-ingress" AND DstK8S_Namespace="netobserv"))
        outputTarget: Loki  # The filter can target a specific output (such as Loki logs or exported data), or all outputs.
        sampling: 10        # Optionally, a sampling interval can be associated with the filter
```

See also the [list of field names](https://github.com/netobserv/network-observability-operator/blob/main/docs/flows-format.adoc) that are available for queries, and the [API documentation](https://github.com/netobserv/network-observability-operator/blob/main/docs/FlowCollector.md#flowcollectorspecprocessorfiltersindex-1).

## Internals

This language is designed using [Yacc](https://en.wikipedia.org/wiki/Yacc) / goyacc.

The [definition file](../pkg/dsl/expr.y) describes the syntax based on a list of tokens. It is derived to a [go source file](../pkg/dsl/expr.y.go) using [goyacc](https://pkg.go.dev/golang.org/x/tools/cmd/goyacc), which defines constants for the tokens, among other things. The [lexer](../pkg/dsl/lexer.go) file defines structures and helpers that can be used from `expr.y`, the logic used to interpret the language in a structured way, and is also where actual characters/strings are mapped to syntax tokens. Finally, [eval.go](../pkg/dsl/eval.go) runs the desired query on actual data.

When adding features to the language, you'll likely have to change `expr.y` and `lexer.go`.

To regenerate `expr.y.go`, run:

```bash
make goyacc
```
