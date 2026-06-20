.PHONY: build test vet proto sqlc tidy clean

SERVICES := gateway compliance-service lookup-service settlement-service quoting-service notification-service audit-service reconciliation-service routing-service certificate-service certgen

build:
	@for svc in $(SERVICES); do \
		echo "building bin/$$svc..."; \
		go build -o "bin/$$svc" "./cmd/$$svc"; \
	done

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

test-integration:
	@echo "Make sure Docker is running (docker ps)"
	go test -v -race -count=1 -timeout=180s ./test/integration/...

load-smoke:
	k6 run test/load/smoke.js

load-stress:
	k6 run test/load/gateway.js

load-soak:
	k6 run test/load/soak.js

clean:
	rm -rf bin/
