# Stage 1: Build frontend
FROM node:24-alpine AS frontend-builder

WORKDIR /src/frontend

# Cache dependencies
COPY frontend/package*.json ./
RUN npm ci

# Build
COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server

# Stage 3: Runtime
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

WORKDIR /app
COPY --from=builder /app/server .
COPY --from=frontend-builder /src/frontend/dist/ ./dist/

# Copy migrations for docker-compose migrate step
COPY migrations/ ./migrations/

USER appuser

EXPOSE 8080

ENTRYPOINT ["./server"]
