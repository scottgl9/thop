# thop Makefile (Rust)

BINARY_NAME=thop
BUILD_DIR=target

.PHONY: all build release test clean fmt clippy check install

all: build

# Build debug binary
build:
	cargo build
	@echo "Binary built: $(BUILD_DIR)/debug/$(BINARY_NAME)"

# Build release binary
release:
	cargo build --release
	@echo "Binary built: $(BUILD_DIR)/release/$(BINARY_NAME)"
	@ls -lh $(BUILD_DIR)/release/$(BINARY_NAME)

# Run tests
test:
	cargo test

# Run tests with output
test-verbose:
	cargo test -- --nocapture

# Format code
fmt:
	cargo fmt

# Check formatting
fmt-check:
	cargo fmt -- --check

# Run clippy lints
clippy:
	cargo clippy -- -D warnings

# Check code (compile without producing binaries)
check:
	cargo check

# Clean build artifacts
clean:
	cargo clean

# Install to ~/.cargo/bin
install: release
	cargo install --path .

# Run in debug mode
run:
	cargo run

# Run with proxy mode
run-proxy:
	cargo run -- --proxy

# Run with status
run-status:
	cargo run -- --status

# Build documentation
doc:
	cargo doc --no-deps --open

# Update dependencies
update:
	cargo update

# Show binary size
size: release
	@echo "Binary size:"
	@ls -lh $(BUILD_DIR)/release/$(BINARY_NAME)
	@echo ""
	@echo "Stripped size (estimate):"
	@strip -o /tmp/$(BINARY_NAME)_stripped $(BUILD_DIR)/release/$(BINARY_NAME)
	@ls -lh /tmp/$(BINARY_NAME)_stripped
	@rm /tmp/$(BINARY_NAME)_stripped

help:
	@echo "Available targets:"
	@echo "  build        - Build debug binary"
	@echo "  release      - Build optimized release binary"
	@echo "  test         - Run all tests"
	@echo "  test-verbose - Run tests with output"
	@echo "  fmt          - Format code"
	@echo "  fmt-check    - Check code formatting"
	@echo "  clippy       - Run clippy lints"
	@echo "  check        - Check code compilation"
	@echo "  clean        - Remove build artifacts"
	@echo "  install      - Install to ~/.cargo/bin"
	@echo "  run          - Run in debug mode"
	@echo "  run-proxy    - Run in proxy mode"
	@echo "  run-status   - Run status command"
	@echo "  doc          - Build and open documentation"
	@echo "  update       - Update dependencies"
	@echo "  size         - Show binary size"
