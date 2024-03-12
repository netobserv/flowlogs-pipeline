# Update genericmap gRPC

Run the following commands to update `genericmap.pb.go` and `genericmap_grpc.pb.go`:

```bash
$ protoc --go_out=./pkg/pipeline/write/grpc ./proto/genericmap.proto
$ protoc --go-grpc_out=./pkg/pipeline/write/grpc ./proto/genericmap.proto
```