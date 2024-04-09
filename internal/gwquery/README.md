# gRPC Gateway query

The gwquery package contains a slightly modified copy of the `query.go` file from gRPC Gateway, copied from [github.com/grpc-ecosystem/grpc-gateway/blob/main/runtime/query.go](https://github.com/grpc-ecosystem/grpc-gateway/blob/main/runtime/query.go), and is licensed according to [./LICENSE](./LICENSE). The `parseField` method is modified to avoid depending on the global protoregistry, which doesn't work in grpcbridge's case where each target has its own registry.

TODO(renbou): push a fix upstream once this is tested to properly work with grpcbridge
