.PHONY: dev build test clean migrate-up migrate-down migrate-create

# Backend
dev-server:
	go run ./cmd/server

build-server:
	go build -o bin/server ./cmd/server

# Frontend
dev-frontend:
	cd frontend && npm run dev

build-frontend:
	cd frontend && npm run build

# Testing
test-backend:
	go test ./... -v -count=1

test-frontend:
	cd frontend && npm run test

test: test-backend test-frontend

# Database
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

# Dev
dev:
	@echo "Run 'make dev-server' and 'make dev-frontend' in separate terminals"
