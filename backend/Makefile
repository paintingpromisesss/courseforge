GOCACHE ?= $(abspath ../.cache/go-build)

.PHONY: run run-pprof test swagger mcp

run: swagger
	go run -tags swagger ./cmd/server

run-pprof: swagger
	GOCACHE=$(GOCACHE) go run -tags "swagger pprof" ./cmd/server

test:
	GOCACHE=$(GOCACHE) go test ./...

mcp:
	GOCACHE=$(GOCACHE) go build -o ../bin/courseforge-mcp ./cmd/mcp

swagger:
	go run github.com/swaggo/swag/cmd/swag init -g main.go -d ./cmd/server,./internal/api/handlers,./internal/api/dto -o ./docs --exclude ./courses

