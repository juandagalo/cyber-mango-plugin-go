# Cyber Mango Plugin

A [Claude Code](https://docs.anthropic.com/en/docs/claude-code) plugin that provides a cyberpunk-themed kanban board manageable by Claude agents via MCP tools.

This is the **plugin** — it gives Claude Code the ability to create, move, and manage kanban cards programmatically. For the **web UI** (human-facing board), see [cyber-mango](https://github.com/juandagalo/cyber-mango).

Both the plugin and the web UI share the same SQLite database, so changes made by Claude appear instantly in the browser and vice versa.

## What's Included

| Component | Description |
|-----------|-------------|
| **MCP Server** | 9 tools for board management via stdio JSON-RPC |
| **Skills** | Markdown protocols that teach Claude *when* and *how* to use the tools |
| **Hooks** | SessionStart (board summary) and Stop (activity recap) lifecycle hooks |

## Why Go

The original TypeScript version used `better-sqlite3`, a native C++ module. Claude Code's plugin cache copies files but doesn't run `npm rebuild`, so the compiled binary was missing and the MCP server crashed on every plugin load. Go compiles to a single static binary — no native modules, no `node_modules`, no shell wrappers.

## Tools

| Tool | Description |
|------|-------------|
| `list_boards` | List all kanban boards |
| `get_board` | Get a board with all columns, cards, and tags |
| `get_board_summary` | Card counts by column and priority |
| `create_card` | Create a card (resolves column by ID or name) |
| `update_card` | Update title, description, or priority |
| `move_card` | Move a card to a different column or position |
| `delete_card` | Delete a card |
| `create_column` | Create a new column on a board |
| `manage_tags` | Create, assign, remove, list, or delete tags |

## Skills

- **board-manage** — Defines when to create/move/update cards, column workflow, priority conventions, WIP limit enforcement, and card description standards.
- **ticket-track** — Syncs external tickets (GitHub issues, Jira, Linear) with the kanban board using a naming convention and cross-session memory.

## Installation

### Prerequisites

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) installed
- [Go 1.21+](https://go.dev/dl/) installed

### Build

```bash
git clone https://github.com/juandagalo/cyber-mango-plugin-go.git
cd cyber-mango-plugin-go
make build
```

This produces three binaries in `bin/`: `mcp-server`, `session-start`, `session-stop` (with `.exe` on Windows). The Makefile detects the OS automatically.

If you don't have `make`, build manually:

```bash
mkdir -p bin
go build -o bin/mcp-server ./cmd/mcp-server
go build -o bin/session-start ./cmd/session-start
go build -o bin/session-stop ./cmd/session-stop
```

### Install as Plugin

```bash
claude plugin marketplace add /path/to/cyber-mango-plugin-go
claude plugin install cyber-mango
```

Restart your Claude Code session. Verify with:

```bash
claude mcp list
# Should show: plugin:cyber-mango:cyber-mango — ✓ Connected
```

## Shared Database

The plugin and the [web UI](https://github.com/juandagalo/cyber-mango) share the same SQLite database. The database path is resolved in this order:

1. `CYBER_MANGO_DB_PATH` environment variable
2. `CLAUDE_PLUGIN_DATA/kanban.db` (set by Claude Code for plugins)
3. `~/.cyber-mango/kanban.db` (default shared location)

On first run, if no boards exist, a default **Cyber Mango** board is created with five columns: Backlog, To Do, In Progress, Review, Done.

## Project Structure

```
cyber-mango-plugin-go/
├── .claude-plugin/          # Plugin + marketplace metadata
├── .mcp.json                # MCP server config (stdio)
├── hooks/hooks.json         # Lifecycle hook definitions
├── skills/                  # board-manage + ticket-track
├── cmd/
│   ├── mcp-server/          # MCP server entry point
│   ├── session-start/       # SessionStart hook
│   └── session-stop/        # Stop hook
├── internal/
│   ├── db/                  # Connection, migrations, seed
│   ├── models/              # Data structs
│   ├── services/            # Business logic
│   └── mcp/                 # Tool handlers + server registration
├── go.mod
├── Makefile
└── README.md
```

## Tech Stack

- **Go** — single binary, no CGo
- **modernc.org/sqlite** — pure Go SQLite
- **github.com/mark3labs/mcp-go** — MCP SDK (stdio transport)
- **github.com/jmoiron/sqlx** — SQL query ergonomics
- **github.com/matoous/go-nanoid/v2** — ID generation

## Testing

```bash
go test ./...
```

Tests use in-memory SQLite (`:memory:`) — no external dependencies.

## Author

Daniel Garcia ([juandagalo](https://github.com/juandagalo))
