.PHONY: all dev dev-server dev-web build build-web build-server clean clean-data reset-data docker-up docker-down free-port-8080

APP_NAME = allinone-server
GO ?= /usr/local/go/bin/go
BUN ?= bun
DB_PATH ?= data/allinone.db
JWT_SECRET ?= dev-secret-change-me
SESSION_SECRET ?= dev-session-secret-change-me
PUID ?= $(shell id -u)
PGID ?= $(shell id -g)

all: build

# 释放本地开发默认后端端口。仅用于本项目开发端口 8080。
free-port-8080:
	@pids=$$(ss -ltnp 2>/dev/null | awk '/:8080 / { if (match($$0, /pid=[0-9]+/)) { print substr($$0, RSTART+4, RLENGTH-4) } }' | sort -u); \
	if [ -n "$$pids" ]; then \
		echo "Killing process(es) on port 8080: $$pids"; \
		kill $$pids 2>/dev/null || true; \
		sleep 1; \
		kill -9 $$pids 2>/dev/null || true; \
	fi

# 启动本地开发环境 (前后端一起跑)
dev: free-port-8080
	@echo "Starting Backend and Frontend in dev mode..."
	@mkdir -p data
	@bash -c 'set -e; \
		ALLINONEKEY_DB_PATH="$(DB_PATH)" ALLINONEKEY_JWT_SECRET="$(JWT_SECRET)" ALLINONEKEY_SESSION_SECRET="$(SESSION_SECRET)" "$(GO)" run ./cmd/server/main.go & \
		server_pid=$$!; \
		trap "kill $$server_pid 2>/dev/null || true" EXIT; \
		cd web && "$(BUN)" run dev & \
		wait'

# 单独启动后端
dev-server: free-port-8080
	@mkdir -p data
	@ALLINONEKEY_DB_PATH="$(DB_PATH)" ALLINONEKEY_JWT_SECRET="$(JWT_SECRET)" ALLINONEKEY_SESSION_SECRET="$(SESSION_SECRET)" "$(GO)" run ./cmd/server/main.go

# 单独启动前端热更新
dev-web:
	cd web && "$(BUN)" run dev

# 编译前后端生产环境产物
build: build-web build-server

build-web:
	@echo "Building frontend..."
	cd web && "$(BUN)" run build

build-server:
	@echo "Building backend..."
	"$(GO)" build -o $(APP_NAME) ./cmd/server/main.go

# 清理构建产物
clean:
	@echo "Cleaning up build artifacts..."
	rm -f $(APP_NAME)
	rm -rf web/dist

# 清理本地开发数据库。用于忘记测试账号 / Master Key 后重置环境。
clean-data: docker-down free-port-8080
	@echo "Cleaning local database files under data/..."
	@mkdir -p data
	rm -f "$(DB_PATH)" "$(DB_PATH)-shm" "$(DB_PATH)-wal"
	@echo "Local database cleaned. Next start will create a fresh database."

# 清理数据并重新启动本地开发环境。
reset-data: clean-data dev

# Docker 快捷命令
docker-up:
	@mkdir -p data
	@if [ -z "$(ALLINONEKEY_JWT_SECRET)" ]; then echo "ALLINONEKEY_JWT_SECRET is required for docker-up"; exit 1; fi
	@if [ -z "$(ALLINONEKEY_SESSION_SECRET)" ]; then echo "ALLINONEKEY_SESSION_SECRET is required for docker-up"; exit 1; fi
	PUID=$(PUID) PGID=$(PGID) docker-compose up --build -d
	docker-compose ps

docker-down:
	docker-compose down
