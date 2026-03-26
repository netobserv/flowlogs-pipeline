# Update genericmap gRPC

Run the following commands to update `genericmap.pb.go` and `genericmap_grpc.pb.go`:

```bash
$ protoc --go_out=./pkg/pipeline/write/grpc ./proto/genericmap.proto
$ protoc --go-grpc_out=./pkg/pipeline/write/grpc ./proto/genericmap.proto
```

# Update k8scache gRPC

Run the following commands to update `k8scache.pb.go` and `k8scache_grpc.pb.go`:

```bash
$ protoc --go_out=./pkg/pipeline/transform/kubernetes/k8scache --go_opt=paths=source_relative ./proto/k8scache.proto
$ protoc --go-grpc_out=./pkg/pipeline/transform/kubernetes/k8scache --go-grpc_opt=paths=source_relative ./proto/k8scache.proto
```