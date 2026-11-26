# Convenience targets. CI runs the same commands (see .github/workflows/ci.yml).
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/Szmelc-INC/Password123/internal/cli.Version=$(VERSION)
PREFIX  ?= /usr/local

.PHONY: build test validate vectors site serve bundle portal-test completions install clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o p123 ./cmd/p123

test:
	go test ./cmd/... ./internal/...

validate: build
	./p123 validate

vectors: build
	./p123 vectors verify

site: build
	./p123 pages build

serve: build
	./p123 pages serve

bundle:
	cd web && npm ci --no-audit --no-fund && npm run build

portal-test:
	cd web && npm test

completions: build
	./p123 completion bash > completions/p123.bash
	./p123 completion zsh > completions/_p123
	./p123 completion fish > completions/p123.fish

install: build
	install -Dm755 p123 $(DESTDIR)$(PREFIX)/bin/p123
	install -Dm644 completions/p123.bash $(DESTDIR)$(PREFIX)/share/bash-completion/completions/p123
	install -Dm644 completions/_p123 $(DESTDIR)$(PREFIX)/share/zsh/site-functions/_p123
	install -Dm644 completions/p123.fish $(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/p123.fish

clean:
	rm -rf p123 _site .tmp result dist
