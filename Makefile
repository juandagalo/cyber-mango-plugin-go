.PHONY: build clean test

BINDIR = bin

# Always use the .exe suffix so .mcp.json and hooks.json (which cannot branch
# per OS) reference one binary name everywhere. Linux/macOS run it fine.
EXT = .exe

build:
	go build -o $(BINDIR)/mcp-server$(EXT) ./cmd/mcp-server
	go build -o $(BINDIR)/session-start$(EXT) ./cmd/session-start
	go build -o $(BINDIR)/session-stop$(EXT) ./cmd/session-stop

test:
	go test ./...

clean:
	rm -rf $(BINDIR)
