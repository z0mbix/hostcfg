# Show available recipes
default:
    @just --list

# Build the binary
build:
    go build -o bin/hostcfg ./cmd/hostcfg

# Build with version info
build-release version:
    go build -ldflags "-X main.version={{ version }}" -o bin/hostcfg ./cmd/hostcfg

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-verbose:
    go test -v ./...

# Run tests with coverage
test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Run a specific package's tests
test-pkg pkg:
    go test -v ./internal/{{ pkg }}/...

# Run linter
lint:
    golangci-lint run

# Format code
fmt:
    go fmt ./...

# Tidy dependencies
tidy:
    go mod tidy

# Clean build artifacts
clean:
    rm -f bin/hostcfg coverage.out coverage.html

# Install binary to GOPATH/bin
install:
    go install ./cmd/hostcfg

# Run plan on example config
plan-example:
    go run ./cmd/hostcfg plan -c ./examples/basic.hcl

# Validate example configs
validate-examples:
    @for f in ./examples/*.hcl; do echo "Validating $f..."; go run ./cmd/hostcfg validate -c "$f"; done

# Build and test
check: fmt test lint

# Watch for changes and run tests (requires watchexec)
watch:
    watchexec -e go -- just test
