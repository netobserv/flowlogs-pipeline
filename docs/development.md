# Development

## Benchmarking

It is possible to benchmark the flowlogs-pipeline using:
```bash
make benchmark
```  

This will execute the flowlogs-pipeline `cmd/flowlogs-pipeline/benchmark_test.go` benchmarks. 
The benchmarks execute flowlogs-pipeline with different configurations and measure execution time.
In addition, CPU profile information is generated and saved into `/tmp/flowlogs-pipeline-cpu-profile.out` 
it is possible to observe the CPU profile information by executing the command:
```bash
go tool pprof -web /tmp/flowlogs-pipeline-cpu-profile.out
```
> Note: installed graphviz is a pre-req. 

## OCP Flow logs correctness

use the script `hack/deploy-and-monitor-k8s-network-workload.sh` to deploy on OCP synthetic demo workloads 
and `flowlogs-pipeline` components to observe and check correctness for the raw flow-logs generated by OCP. 

The script deploys three workloads:  
(1) egress from the cluster   
(2) ingress to the cluster  
(3) pod-to-pod traffic  

Each workload is deployed into separate namespace to ease monitoring.
The script also deploys `flowlogs-pipeline` on OCP with specific configuration that doesn't include any 
aggregation and/or export to prometheus, just simple dump of the logs into stdout after enrichment with 
k8s information (e.g. pod-name, namespace etc.). This allows usage of simple tools, such as `grep` to
focus on specific flow-logs generated by OCP.

## Edit diagrams
- To edit drawio diagrams (with .png extension) use [https://app.diagrams.net/](https://app.diagrams.net/)

