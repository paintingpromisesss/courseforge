GOCACHE ?= ../.cache/go-build

.PHONY: run test swagger

run: swagger
	GOCACHE=$(GOCACHE) go run -tags swagger ./cmd/server

test:
	GOCACHE=$(GOCACHE) go test ./...

swagger:
	GOCACHE=$(GOCACHE) go run github.com/swaggo/swag/cmd/swag init -g main.go -d ./cmd/server,./internal/api/handlers,./internal/api/dto -o ./docs --exclude ./courses
