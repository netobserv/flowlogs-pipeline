# Regenerating protobuf / gRPC code

After editing any `.proto` file in this directory, regenerate the Go code with:

```bash
$ make proto
```

The target is self-contained: it downloads the pinned `protoc` release archive into
`./bin/protoc-<version>/` (binary and its bundled `google/protobuf/*.proto`
well-known types side by side, so imports resolve automatically) and installs the
pinned `protoc-gen-go` / `protoc-gen-go-grpc` plugins into `./bin`. Nothing needs to
be installed system-wide.

Pinning everything (see `PROTOC_VERSION`, `PROTOC_GEN_GO_VERSION` and
`PROTOC_GEN_GO_GRPC_VERSION` in the `Makefile`) keeps the generated code
reproducible: regenerating with a different toolchain would otherwise produce
spurious diffs. When bumping any of those versions, run `make proto` and commit the
regenerated `*.pb.go` files in the same change.

## Manual commands

For reference, `make proto` runs the following (using the downloaded `$(PROTOC)`):

### genericmap gRPC

Regenerates `genericmap.pb.go` and `genericmap_grpc.pb.go`:

```bash
$ protoc --go_out=./pkg/pipeline/write/grpc ./proto/genericmap.proto
$ protoc --go-grpc_out=./pkg/pipeline/write/grpc ./proto/genericmap.proto
```

### k8scache gRPC

Regenerates `k8scache.pb.go` and `k8scache_grpc.pb.go`:

```bash
$ protoc --go_out=./pkg/pipeline/transform/kubernetes/k8scache --go_opt=paths=source_relative -I ./proto k8scache.proto
$ protoc --go-grpc_out=./pkg/pipeline/transform/kubernetes/k8scache --go-grpc_opt=paths=source_relative -I ./proto k8scache.proto
```
