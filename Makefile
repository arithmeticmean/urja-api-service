-include .env
export

GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.24.1

build:
	go build -o bin/urja-ops-api-service .

run: build
	./bin/urja-ops-api-service

test:
	go test ./... -v

migrate:
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL)" down

migration:
	$(GOOSE) -dir migrations create $(name) sql

clean:
	rm -rf bin
