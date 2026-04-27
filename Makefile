.PHONY: build test lint check check-web run sync-static

build: sync-static
	go build -o bin/picooraclaw-webui ./cmd/picooraclaw-webui

test:
	go test ./...

lint:
	go vet ./...

# Mirrors what CI runs. Run `make check` before pushing to catch the same
# errors locally (svelte-check has burned a string of red builds otherwise).
check: lint test check-web

check-web:
	cd web && npm install --no-audit --no-fund && npm run check

run: build
	./bin/picooraclaw-webui --picooraclaw-url http://localhost:8090 --listen :3000

sync-static:
	@if [ -d web/build ] && [ -n "$$(ls -A web/build 2>/dev/null | grep -v .gitkeep)" ]; then \
	    rm -rf cmd/picooraclaw-webui/static; \
	    mkdir -p cmd/picooraclaw-webui/static; \
	    cp -r web/build/* cmd/picooraclaw-webui/static/; \
	    touch cmd/picooraclaw-webui/static/.gitkeep; \
	fi
