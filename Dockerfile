# Stage 1: build frontend
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: build Go binary
FROM golang:1.24-alpine AS go
WORKDIR /src
RUN apk add --no-cache make
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/build ./cmd/picooraclaw-webui/static
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /picooraclaw-webui ./cmd/picooraclaw-webui

# Stage 3: runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=go /picooraclaw-webui /usr/local/bin/picooraclaw-webui
EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/picooraclaw-webui"]
