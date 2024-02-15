FROM golang:1.22.0-alpine3.19 AS builder

WORKDIR /build
COPY . .
RUN \
  --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go build -o /build/testapi

FROM alpine
COPY --from=builder /build/testapi /bin/testapi
WORKDIR /opt/testapi

CMD [ "/bin/testapi" ]
