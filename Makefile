.PHONY: build test vet proto sqlc tidy clean

build:
	go build -o bin/gateway ./cmd/gateway

test:
	go test ./... -race -count=1

vet:
	go vet ./...

proto:
	protoc --go_out=. --go_opt=module=switch \
	       --go-grpc_out=. --go-grpc_opt=module=switch \
	       -I api/proto \
	       api/proto/*.proto

sqlc:
	sqlc generate

tidy:
	go mod tidy

clean:
	rm -rf bin/
