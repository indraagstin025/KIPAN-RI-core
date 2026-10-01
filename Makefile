.PHONY: run build test lint migrate-up migrate-down tidy keys

# Jalankan server dengan hot reload (butuh: go install github.com/air-verse/air@latest)
run:
	air

# Build binary production (SATU entrypoint: ./cmd/api)
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/kipan-api ./cmd/api

# Jalankan semua test
test:
	go test ./... -race -count=1 -v

# Lint (butuh: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
	golangci-lint run ./...

# Go mod tidy
tidy:
	go mod tidy

# Migrate UP (butuh: DATABASE_URL di environment)
# BE-009: -tags 'postgres file' wajib — tanpa itu CLI error
# `unknown driver postgres (forgotten import?)`.
migrate-up:
	go run -tags 'postgres file' -mod=mod github.com/golang-migrate/migrate/v4/cmd/migrate -path migrations -database "$${DATABASE_URL}" up

# Migrate DOWN 1 step
migrate-down:
	go run -tags 'postgres file' -mod=mod github.com/golang-migrate/migrate/v4/cmd/migrate -path migrations -database "$${DATABASE_URL}" down 1

# Versi migrasi saat ini (diagnostik)
migrate-version:
	go run -tags 'postgres file' -mod=mod github.com/golang-migrate/migrate/v4/cmd/migrate -path migrations -database "$${DATABASE_URL}" version

# Generate 3 kunci kriptografi secara aman
keys:
	@echo "=== Generate Crypto Keys (copy ke .env) ==="
	@echo "AES_MASTER_KEY=$$(openssl rand -hex 32)"
	@echo "BLIND_INDEX_KEY=$$(openssl rand -hex 32)"
	@echo "KTA_SIGNING_KEY=$$(openssl rand -hex 32)"
	@echo "AUTH_ACCESS_TOKEN_SECRET=$$(openssl rand -hex 32)"

.PHONY: seed
seed:
	@echo "🌱 Seeding database..."
	go run ./cmd/seed