FROM rust:1.77.1 AS builder

WORKDIR /build

RUN apt update && apt install -y protobuf-compiler libprotobuf-dev

COPY . .
RUN \
  --mount=type=cache,target=/usr/local/cargo/registry \
  --mount=type=cache,target=/build/target \
  cargo build --release && \
  cp target/release/rstestapi .

FROM ubuntu:23.10
COPY --from=builder /build/rstestapi /bin/rstestapi
WORKDIR /opt/rstestapi

CMD [ "/bin/rstestapi" ]

