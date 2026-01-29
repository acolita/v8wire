# v8wire - Go library for V8 serialization format
.PHONY: all test test-compat generate-fixtures clean lint

all: test

# Run all tests
test:
	go test -v ./...

# Run only compatibility tests
test-compat:
	go test -v -run TestCrossVersionCompatibility ./pkg/v8serialize/

# Generate fixtures for the current Node.js version (local)
generate-fixtures:
	cd testgen && npm install && node generate.js

# Generate fixtures for all Node.js versions using Docker
generate-all-fixtures:
	cd testgen && ./generate-all.sh

# Run fuzz tests (short)
fuzz:
	go test -fuzz=FuzzDeserialize -fuzztime=30s ./pkg/v8serialize/

# Run fuzz tests (long)
fuzz-long:
	go test -fuzz=FuzzDeserialize -fuzztime=5m ./pkg/v8serialize/

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Clean generated files
clean:
	rm -f testdata/fixtures/*.bin testdata/fixtures/*.json
	rm -rf testdata/fixtures/v13 testdata/fixtures/v14 testdata/fixtures/v15

# Lint code
lint:
	go vet ./...
	@which golangci-lint > /dev/null && golangci-lint run || echo "golangci-lint not installed"

# Run tests and show coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Verify Node.js fixtures are up-to-date
verify:
	cd testgen && node verify.js

# Show help
help:
	@echo "v8wire Makefile targets:"
	@echo "  make test              - Run all tests"
	@echo "  make test-compat       - Run cross-version compatibility tests"
	@echo "  make generate-fixtures - Generate fixtures with local Node.js"
	@echo "  make generate-all-fixtures - Generate fixtures for all Node.js versions (Docker)"
	@echo "  make fuzz              - Run fuzz tests (30s)"
	@echo "  make fuzz-long         - Run fuzz tests (5m)"
	@echo "  make bench             - Run benchmarks"
	@echo "  make coverage          - Generate coverage report"
	@echo "  make clean             - Remove generated fixtures"
	@echo "  make lint              - Run linters"
