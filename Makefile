all: build test

build:
	@echo "Building..."
	
	@go build -o sylius-mcp .

test:
	@echo "Testing..."
	@go test ./... -v

.PHONY: all build test
