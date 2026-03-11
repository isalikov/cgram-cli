.PHONY: build run dev clean test vendor help

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build    Build binary to ./bin/cgram"
	@echo "  run      Run without building (go run)"
	@echo "  dev      Run with .env loaded"
	@echo "  clean    Remove ./bin directory"
	@echo "  test     Run tests"
	@echo "  vendor   Download dependencies to vendor/"
	@echo "  help     Show this help"

build:
	go build -o ./bin/cgram ./cmd/cgram

run:
	go run ./cmd/cgram

dev:
	go run ./cmd/cgram

clean:
	rm -rf ./bin

test:
	go test ./...

vendor:
	go mod tidy
	go mod vendor
