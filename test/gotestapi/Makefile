ROOT_DIR ?= $(CURDIR)
BIN_DIR ?= $(ROOT_DIR)/bin

.PHONY: .gen-deps
.gen-deps: export GOBIN := $(BIN_DIR)
.gen-deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32.0
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

.PHONY: gen
gen: export PATH := $(BIN_DIR):$(PATH)
gen: .gen-deps
	buf generate
