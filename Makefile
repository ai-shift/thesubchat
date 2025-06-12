.PHONY: format lint
include .env.example
-include .env

format:
	sleek ./sql/migrations/*
	sleek ./sql/queries/*
	golangci-lint fmt ./...

lint: format
	go vet ./...
	staticcheck ./...

sqlc-generate: format
	sqlc generate

migrate-up:
	geni up

# make migrate-new NAME=<your migration name>
migrate-new:
	geni new $(NAME)

install:
	wget -O $(HOME)/.local/bin/sleek https://github.com/nrempel/sleek/releases/download/v0.5.0/sleek-linux-x86_64
	chmod +x $(HOME)/.local/bin/sleek
	curl -fsSL -o $(HOME)/.local/bin/geni https://github.com/emilpriver/geni/releases/download/v1.1.6/geni-linux-amd64
	chmod +x $(HOME)/.local/bin/geni
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/air-verse/air@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

run: lint
	go run ./cmd/server/main.go

run-air: lint
	air --build.cmd "go build -o tmp/app cmd/server/main.go" --build.bin "./tmp/app"
