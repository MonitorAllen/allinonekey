# Build Frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /web
# Install Bun
RUN npm install -g bun
COPY web/package.json web/bun.lockb* ./
RUN bun install
COPY web/ ./
ARG VITE_ALLINONEKEY_APP_VERSION=0.2.0
ENV VITE_ALLINONEKEY_APP_VERSION=$VITE_ALLINONEKEY_APP_VERSION
RUN bun run build

# Build Backend
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server ./cmd/server/main.go

# Final Image
FROM alpine:latest
WORKDIR /app
ENV ALLINONEKEY_APP_VERSION=0.2.0
RUN apk add --no-cache su-exec
COPY --from=backend-builder /app/server .
COPY --from=frontend-builder /web/dist ./web/dist
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
EXPOSE 8080
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["./server"]
