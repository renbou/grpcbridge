ROOT_DIR ?= $(CURDIR)
BIN_DIR ?= $(ROOT_DIR)/bin
export PATH := $(BIN_DIR):$(PATH)

TEST_TIMEOUT = "1m"

.PHONY: zzz.testdeps
zzz.testdeps: export GOBIN := $(BIN_DIR)
zzz.testdeps:
	go install github.com/rakyll/gotest@latest

# Test specified PKG
.PHONY: testpkg
testpkg: PKG = "./..."
testpkg: zzz.testdeps
	gotest -race -timeout $(TEST_TIMEOUT) $(PKG)


# Test & cover specified PKG
.PHONY: coverpkg
coverpkg: PKG = "./..."
coverpkg: zzz.testdeps
	gotest -race -timeout $(TEST_TIMEOUT) -coverprofile coverage.txt -covermode atomic $(PKG)
