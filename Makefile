.PHONY: build test test-v lint fmt run clean smoke

BIN := bin/orchestrator

build:
	go build -o $(BIN) ./cmd/orchestrator

test:
	go test ./...

test-v:
	go test -v ./...

test-race:
	go test -race ./...

fmt:
	go fmt ./...
	goimports -w .

lint:
	go vet ./...

run: build
	./$(BIN)

clean:
	rm -rf bin/ *.db *.db-wal *.db-shm coverage.out

BIN_RUNNER_LINUX := bin/era-runner-linux

runner-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_RUNNER_LINUX) ./cmd/runner

BIN_SIDECAR_LINUX := bin/era-sidecar-linux

sidecar-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(BIN_SIDECAR_LINUX) ./cmd/sidecar

docker-runner: runner-linux sidecar-linux
	docker build -t era-runner:m2 -f docker/runner/Dockerfile .
