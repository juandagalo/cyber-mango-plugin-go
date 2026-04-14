.PHONY: build clean test

BINDIR = bin
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
EXT =
ifeq ($(GOOS),windows)
  EXT = .exe
endif

build:
	go build -o $(BINDIR)/mcp-server$(EXT) ./cmd/mcp-server
	go build -o $(BINDIR)/session-start$(EXT) ./cmd/session-start
	go build -o $(BINDIR)/session-stop$(EXT) ./cmd/session-stop

test:
	go test ./...

clean:
	rm -rf $(BINDIR)
