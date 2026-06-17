SWAG := $(shell go env GOPATH)/bin/swag

.PHONY: run test swagger

run:
	go run ./cmd/server

test:
	go test ./...

swagger:
	$(SWAG) init -g ./cmd/server/main.go -o ./docs --exclude ./courses
