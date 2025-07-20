.PHONY: all build docker-build up down logs test coverage integration-test integration-coverage

IMAGE_NAME = staking-validator-api

all: build

run:
	go run ./cmd/api

build:
	go build -o eth-validator-api ./cmd/api

docker-build:
	docker build -t $(IMAGE_NAME):latest .

up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f

test:
	go test -v ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

integration-test:
	go test -v ./integration -timeout 30s

integration-coverage:
	go test -coverpkg=./internal/... ./integration \
	  -coverprofile=integration.out \
	  -timeout 30s
	go tool cover -html=integration.out -o integration-coverage.html
