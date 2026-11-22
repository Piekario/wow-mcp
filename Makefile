# wow-mcp build tooling. Needs: go (>= 1.25), python3 + lupa and luacheck for `make test`, zip + tar + sha256sum for `make dist`.
# Versions are read from the source (server/main.go serverVersion, addon .toc), never hard-coded here.

SERVER_VERSION := $(shell scripts/version.sh server)
ADDON_VERSION  := $(shell scripts/version.sh addon)
GO             ?= go
EXE            := $(shell $(GO) env GOEXE)

.PHONY: help test test-go test-addon lint build dist version clean

help:
	@echo "targets:"
	@echo "  test   go vet + go test, addon harness vs committed fixtures, luacheck"
	@echo "  build  build the server for this platform into bin/wow-mcp$(EXE)"
	@echo "  dist   cross-compile and package release archives + SHA256SUMS into dist/"
	@echo "  clean  remove dist/ and the built binary"

version:
	@echo "server $(SERVER_VERSION), addon $(ADDON_VERSION)"

test: test-go test-addon lint

test-go:
	cd server && $(GO) vet ./... && $(GO) test ./...

test-addon:
	scripts/check-addon-fixtures.sh

lint:
	luacheck addon

build:
	mkdir -p bin
	cd server && $(GO) build -trimpath -ldflags "-s -w" -o ../bin/wow-mcp$(EXE) .
	@echo "built bin/wow-mcp$(EXE) (server $(SERVER_VERSION))"

dist:
	GO=$(GO) scripts/dist.sh

clean:
	rm -rf dist bin/wow-mcp bin/wow-mcp.exe
