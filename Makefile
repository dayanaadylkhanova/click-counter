.PHONY: tidy build run dev-up dev-down

APP=clicks-api

ldflags = -X 'main.AppName=$(APP)' \
	-X 'main.AppRelease=dev' \
	-X 'main.AppCommit=$$(git rev-parse --short HEAD || echo local)' \
	-X 'main.AppBuildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)'

tidy:
	go mod tidy

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(ldflags)" -o bin/$(APP) ./cmd/clicks-api

run:
	LISTEN_ADDR=:3000 LOG_LEVEL=debug \
	DATABASE_URL="postgres://postgres:postgres@localhost:5432/clicks?sslmode=disable" \
	go run -ldflags "$(ldflags)" ./cmd/clicks-api

dev-up:
	cd dev && docker compose --env-file .env up --build -d

dev-down:
	cd dev && docker compose down -v
