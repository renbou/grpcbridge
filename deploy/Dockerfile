FROM --platform=$BUILDPLATFORM golang:1.22.3-alpine3.19 AS builder

WORKDIR /build
COPY . /build
RUN \
  --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-w -s" -o grpcbridge ./cmd/grpcbridge

# By default alpine is used for easier debugging,
# but /bin/grpcbridge can be copied from this image to any other linux-based image, e.g. scratch/ubuntu
FROM alpine
COPY --from=builder /build/grpcbridge /bin/grpcbridge

# Don't expose any ports or config paths by default, but set a working directory for config files to be attached to it.
WORKDIR /opt/grpcbridge
ENTRYPOINT [ "/bin/grpcbridge" ]
