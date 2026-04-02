.PHONY: run run-worker build fmt test docker-build compose-up

run:
	go run ./cmd/web

run-worker:
	go run ./cmd/worker

build:
	go build ./...

fmt:
	go fmt ./...

test:
	go test ./...

docker-build:
	docker build -t media-pipeline:stage2 .

compose-up:
	docker compose up --build
