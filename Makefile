.PHONY: build test lint run

build:
	go build -o bin/picooraclaw-webui ./cmd/picooraclaw-webui

test:
	go test ./...

lint:
	go vet ./...

run: build
	./bin/picooraclaw-webui --picooraclaw-url http://localhost:8090 --listen :3000
