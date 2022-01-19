# Development

## Benchmarking

It is possible to benchmark the flowlogs2metrics using:
```bash
make benchmark
```  

This will execute the flowlogs2metrics `cmd/flowlogs2metrics/benchmark_test.go` benchmarks. 
The benchmarks execute flowlogs2metrics with different configurations and measure execution time.
In addition, CPU profile information is generated and saved into `/tmp/flowlogs2metrics-cpu-profile.out` 
it is possible to observe the CPU profile information by executing the command:
```bash
go tool pprof -web /tmp/flowlogs2metrics-cpu-profile.out
```
> Note: installed graphviz is a pre-req. 

## Edit diagrams
- To edit drawio diagrams (with .png extension) use [https://app.diagrams.net/](https://app.diagrams.net/)

