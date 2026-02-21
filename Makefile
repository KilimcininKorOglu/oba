.PHONY: build clean test test-race test-cover bench run help docker docker-run docker-stop

BINARY_NAME=oba
BUILD_DIR=bin
CMD_DIR=cmd/oba
DOCKER_IMAGE=oba:latest

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

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
	./$(BUILD_DIR)/$(BINARY_NAME) serve

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

docker:
	docker build -t $(DOCKER_IMAGE) .

docker-run:
	docker run -d --name oba-server \
		-p 1389:1389 -p 8080:8080 \
		-v $(PWD)/docker-data:/var/lib/oba \
		$(DOCKER_IMAGE) ./bin/oba serve --config /var/lib/oba/config.yaml

docker-stop:
	docker rm -f oba-server 2>/dev/null || true

docker-logs:
	docker logs -f oba-server

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
