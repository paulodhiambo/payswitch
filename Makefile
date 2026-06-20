.PHONY: build test lint clean

build:
	go build -o bin/gateway ./cmd/gateway

test:
	go test ./... -race -count=1

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
