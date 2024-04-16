FROM golang:1.22.2-alpine3.19 AS builder

WORKDIR /build
COPY . .
RUN \
  --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go build -o /build/gotestapi

FROM alpine
COPY --from=builder /build/gotestapi /bin/gotestapi
WORKDIR /opt/gotestapi

CMD [ "/bin/gotestapi" ]
