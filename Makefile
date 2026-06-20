.PHONY: build test vet sqlc tidy clean

build:
	go build -o bin/gateway ./cmd/gateway

test:
	go test ./... -race -count=1

vet:
	go vet ./...

sqlc:
	sqlc generate

tidy:
	go mod tidy

clean:
	rm -rf bin/
