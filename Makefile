.PHONY: build clean run vendor

build:
	go build -o cgram ./cmd/cgram

vendor:
	go mod tidy
	go mod vendor

clean:
	rm -f cgram

run:
	go run ./cmd/cgram $(ARGS)
