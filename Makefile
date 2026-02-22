.PHONY: build clean test test-race test-cover test-verbose bench run fmt vet lint help \
	docker docker-run docker-stop docker-logs \
	up down restart logs \
	up-cluster down-cluster restart-cluster logs-cluster clean-cluster-data verify-cluster

BINARY_NAME=oba
BUILD_DIR=bin
CMD_DIR=cmd/oba
DOCKER_IMAGE=oba:latest
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "1.0.0")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-s -w -X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.buildDate=$(BUILD_DATE)'

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)_$(GOOS)_$(GOARCH) ./$(CMD_DIR)

clean:
	rm -rf $(BUILD_DIR)
	go clean

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -cover ./...

test-verbose:
	go test -v ./...

bench:
	go test -bench=. -benchmem ./...

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)_$(GOOS)_$(GOARCH) serve

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(DOCKER_IMAGE) .

docker-run:
	docker run -d --name oba-server \
		-p 1389:1389 -p 8080:8080 \
		-v $(PWD)/docker-data:/var/lib/oba \
		$(DOCKER_IMAGE) ./bin/oba serve --config /var/lib/oba/config.yaml

docker-stop:
	docker rm -f oba-server 2>/dev/null || true

docker-logs:
	docker logs -f oba-server

up:
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_DATE=$(BUILD_DATE) docker compose up -d --build

down:
	docker compose down

restart:
	docker compose restart

logs:
	docker compose logs -f oba

up-cluster:
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_DATE=$(BUILD_DATE) docker compose -f docker-compose.cluster.yml up -d --build

down-cluster:
	docker compose -f docker-compose.cluster.yml down

restart-cluster:
	docker compose -f docker-compose.cluster.yml restart

logs-cluster:
	docker compose -f docker-compose.cluster.yml logs -f

clean-cluster-data:
	find docker-cluster/node1 docker-cluster/node2 docker-cluster/node3 -type f \( -name '*.oba' -o -name '*.dat' -o -name 'raft.log' -o -name 'oba.pid' \) -delete
	find docker-cluster/node1 docker-cluster/node2 docker-cluster/node3 -mindepth 1 -type d -empty -delete

verify-cluster:
	scripts/verify_cluster.sh

help:
	@echo "Available targets:"
	@echo "  build        - Build the binary to bin/"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run all tests"
	@echo "  test-race    - Run tests with race detector"
	@echo "  test-cover   - Run tests with coverage"
	@echo "  test-verbose - Run tests with verbose output"
	@echo "  bench        - Run benchmarks"
	@echo "  run          - Build and run the server"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  lint         - Run fmt and vet"
	@echo "  docker       - Build Docker image"
	@echo "  docker-run   - Run server in Docker container"
	@echo "  docker-stop  - Stop Docker container"
	@echo "  docker-logs  - View Docker container logs"
	@echo "  up           - Build and start all services (docker compose)"
	@echo "  down         - Stop all services (docker compose)"
	@echo "  restart      - Restart all services (docker compose)"
	@echo "  logs         - View server logs (docker compose)"
	@echo "  up-cluster   - Build and start cluster services"
	@echo "  down-cluster - Stop cluster services"
	@echo "  restart-cluster - Restart cluster services"
	@echo "  logs-cluster - View cluster service logs"
	@echo "  clean-cluster-data - Remove cluster DB/log/raft data files"
	@echo "  verify-cluster - Run cluster LDAP/log sync verification"
