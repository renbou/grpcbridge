ROOT_DIR ?= $(CURDIR)
BIN_DIR ?= $(ROOT_DIR)/bin
export PATH := $(BIN_DIR):$(PATH)

TEST_TIMEOUT = "1m"

.PHONY: zzz.testdeps
zzz.testdeps: export GOBIN := $(BIN_DIR)
zzz.testdeps:
	which gotest || go install github.com/rakyll/gotest@latest

# Test specified PKG
.PHONY: testpkg
testpkg: PKG = "./..."
testpkg: zzz.testdeps
	gotest -race -timeout $(TEST_TIMEOUT) $(FLAGS) $(PKG)


# Test & cover specified PKG
.PHONY: coverpkg
coverpkg: PKG = "./..."
coverpkg: zzz.testdeps
	gotest -race -timeout $(TEST_TIMEOUT) -coverprofile coverage.txt -covermode atomic $(FLAGS) $(PKG)

# Show coverage report
showcover: coverpkg
	go tool cover -html coverage.txt -o coverage.html
	open coverage.html

.PHONY: zzz.protodeps
zzz.protodeps: export GOBIN := $(BIN_DIR)
zzz.protodeps:
	which protoc-gen-go || go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0
	which protoc-gen-go-grpc || go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

# Generate protos for tests
.PHONY: proto
proto: zzz.protodeps
proto:
	cd internal/bridgetest/testpb && buf generate
