.PHONY: proto build lint up down logs infra run-%

PATH := $(PATH):$(HOME)/go/bin

proto:            ## regenerate protobuf / gRPC / grpc-gateway code
	buf dep update && buf generate

build:
	go build ./...

lint:
	golangci-lint run ./...

infra:            ## only postgres + kafka + redis (run services with `make run-<svc>`)
	docker compose up -d postgres kafka redis

up:               ## whole stack, UI at http://localhost:8080
	docker compose up -d --build

down:
	docker compose down -v

logs:
	docker compose logs -f --tail=100

run-%:
	go run ./cmd/$*
