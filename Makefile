PREFIX ?= /usr/local

.PHONY: build install uninstall run dev clean test vendor help

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build      Build binary to ./bin/cgram"
	@echo "  install    Build and install to $(PREFIX)/bin/cgram"
	@echo "  uninstall  Remove from $(PREFIX)/bin/cgram"
	@echo "  run        Run without building (go run)"
	@echo "  dev        Run with .env loaded"
	@echo "  clean      Remove ./bin directory"
	@echo "  test       Run tests"
	@echo "  vendor     Download dependencies to vendor/"
	@echo "  help       Show this help"

build:
	go build -o ./bin/cgram ./cmd/cgram

install: build
	sudo install -d $(PREFIX)/bin
	sudo install -m 755 ./bin/cgram $(PREFIX)/bin/cgram

uninstall:
	sudo rm -f $(PREFIX)/bin/cgram

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
