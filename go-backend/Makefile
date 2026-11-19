.PHONY: dev build run clean migrate frontend check-hosts

# Pagar dependensi luar: gagal bila ada host di luar scripts/external-hosts.allowlist.
# Tambahkan --dist setelah `pnpm build` untuk ikut memeriksa hasil build.
check-hosts:
	node scripts/check-external-hosts.mjs

# Go backend
dev:
	GIN_MODE=debug go run ./cmd/server

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/server ./cmd/server

run: build
	./bin/server

clean:
	rm -rf ./bin

# Frontend
frontend-install:
	cd web && pnpm install

frontend-dev:
	cd web && pnpm dev

frontend-build:
	cd web && pnpm build

# Docker
docker-build:
	docker build -t wagataway .

docker-up:
	docker compose up -d

docker-down:
	docker compose down
