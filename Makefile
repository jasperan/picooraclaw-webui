.PHONY: build test lint run sync-static

build: sync-static
	go build -o bin/picooraclaw-webui ./cmd/picooraclaw-webui

test:
	go test ./...

lint:
	go vet ./...

run: build
	./bin/picooraclaw-webui --picooraclaw-url http://localhost:8090 --listen :3000

sync-static:
	@if [ -d web/build ] && [ -n "$$(ls -A web/build 2>/dev/null | grep -v .gitkeep)" ]; then \
	    rm -rf cmd/picooraclaw-webui/static; \
	    mkdir -p cmd/picooraclaw-webui/static; \
	    cp -r web/build/* cmd/picooraclaw-webui/static/; \
	fi
