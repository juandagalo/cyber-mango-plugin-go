# Cyber Mango Plugin (Go)

Claude Code plugin that provides a cyberpunk-themed kanban board manageable by Claude agents via MCP tools. Go rewrite of the original TypeScript version — single static binary, no CGo, no npm.

## Quick Reference

- **Version**: 0.2.0
- **Module**: `github.com/juandagalo/cyber-mango-plugin-go`
- **Go**: 1.23+
- **MCP SDK**: `github.com/mark3labs/mcp-go` v0.44.0
- **SQLite**: `modernc.org/sqlite` (pure Go, no CGo)
- **SQL**: `github.com/jmoiron/sqlx`
- **IDs**: `github.com/matoous/go-nanoid/v2` (12-char nanoid)
- **License**: MIT

## Build

```bash
make build        # produces bin/mcp-server, bin/session-start, bin/session-stop
make test         # go test ./...
make clean        # rm -rf bin/
```

On Windows without `make` in PATH (common in git bash), build manually:

```bash
go build -o bin/mcp-server.exe ./cmd/mcp-server
go build -o bin/session-start.exe ./cmd/session-start
go build -o bin/session-stop.exe ./cmd/session-stop
```

Do NOT run `make build` after code changes automatically — only build when explicitly asked.

## Architecture

### Binaries (cmd/)

| Binary | Entry Point | Purpose |
|--------|-------------|---------|
| `mcp-server` | `cmd/mcp-server/main.go` | MCP server over stdio (JSON-RPC). Opens DB, runs migrations, seeds default board, serves tools. |
| `session-start` | `cmd/session-start/main.go` | SessionStart hook. Outputs board summary as `{"systemMessage": "..."}` JSON to stdout. Silent exit on any error. |
| `session-stop` | `cmd/session-stop/main.go` | Stop hook. Outputs activity summary (last 30 min) as `{"systemMessage": "..."}` JSON. Silent exit if no activity. |

### Internal Packages (internal/)

| Package | Files | Purpose |
|---------|-------|---------|
| `db` | `connection.go`, `migration.go`, `seed.go`, `db_test.go` | DB connection with pragmas (WAL, FK, busy_timeout), schema migration (versioned via `_meta` table), default board seed. |
| `models` | `models.go` | Data structs: Board, Column, Card, Phase, Tag, ActivityLog, BoardSummary, ColumnSummary. All use `db:` and `json:` struct tags. |
| `services` | `board_service.go`, `card_service.go`, `column_service.go`, `tag_service.go`, `phase_service.go`, `activity_service.go`, `services_test.go` | Business logic. All functions take `*sqlx.DB` as first arg (no service structs). Activity logging on every write operation. |
| `mcp` | `server.go`, `handlers.go` | MCP tool registration and handler dispatch. `Handlers` struct holds `*sqlx.DB`. Uses `req.GetString(key, "")` (mcp-go v0.44.0 API). |

### Plugin Metadata (.claude-plugin/)

| File | Purpose |
|------|---------|
| `plugin.json` | Plugin identity: name, version, description, author. |
| `marketplace.json` | Self-contained marketplace. `source: "./"` makes the plugin its own marketplace. Root schema: only `name`, `owner`, `plugins` (no description/version at root). |

### MCP Config (.mcp.json)

Uses `${CLAUDE_PLUGIN_ROOT}` to resolve binary paths. Passes `CYBER_MANGO_DB_PATH` env var.

### Hooks (hooks/)

| Event | Binary/Command | Timeout | Output |
|-------|----------------|---------|--------|
| `SessionStart` | `session-start.exe` | 10s | Board summary (card counts, priority alerts, phase breakdown) |
| `Stop` | `session-stop.exe` | 5s | Activity summary (card + phase actions) |
| `PostToolUse` (mem_save) | inline echo | 3s | Reminder to check if board needs updating after engram save |

### Skills (skills/)

| Skill | File | Trigger |
|-------|------|---------|
| `board-manage` | `skills/board-manage/SKILL.md` | Any work item, task, or board management context |
| `ticket-track` | `skills/ticket-track/SKILL.md` | External ticket references (GitHub issues, Jira, Linear) |

## Database

### Path Resolution (in order)

1. `CYBER_MANGO_DB_PATH` env var
2. `~/.cyber-mango/kanban.db` (default shared location)

`CLAUDE_PLUGIN_DATA` is intentionally NOT used — hooks cannot reliably access it (no `env` field in `hooks.json`, inline `${VAR}` substitution broken for SessionStart), which causes MCP server and hooks to diverge to different DBs.

The `isResolved()` guard in `connection.go` rejects unexpanded template strings like `${VAR}` — Claude Code passes these literally when the underlying env var is not set.

### Schema (7 tables + meta)

- `boards` — id, name, description, timestamps
- `columns` — id, board_id (FK), name, color, wip_limit, position (REAL), timestamps
- `phases` — id, board_id (FK), name, color, position (REAL), timestamps. Unique index on (board_id, name).
- `cards` — id, column_id (FK), title, description, priority (CHECK: low/medium/high/critical), position (REAL), parent_card_id, due_date, phase_id (FK nullable, ON DELETE SET NULL), timestamps
- `tags` — id, board_id (FK), name, color. Unique index on (board_id, name).
- `card_tags` — card_id + tag_id (composite PK, both FK with CASCADE)
- `activity_log` — id, board_id (FK), card_id, action, details, agent, timestamp
- `_meta` — key/value for schema versioning (current: "2")
- `__drizzle_migrations` — Drizzle ORM journal (seeded by Go plugin so web UI recognizes schema)

### Pragmas (applied on every Open)

- `journal_mode = WAL`
- `busy_timeout = 5000`
- `foreign_keys = ON`
- `synchronous = NORMAL`

### Seed

On first run (0 boards), creates a "Cyber Mango" board with 5 columns: Backlog (pos 1000), To Do (2000), In Progress (3000), Review (4000), Done (5000). Also seeds 5 default phases: Development (#00FFFF), Code Review (#BF00FF), QA (#FCEE0A), Client Review (#FF00FF), Ready to Deploy (#39FF14).

## MCP Tools (10)

| Tool | Required Params | Optional Params |
|------|----------------|-----------------|
| `list_boards` | — | — |
| `get_board` | — | board_id |
| `get_board_summary` | — | board_id |
| `create_card` | title | column_id, column_name, board_id, description, priority, tags, phase_id, phase_name |
| `update_card` | card_id | title, description, priority, phase_id, phase_name, unset_phase |
| `move_card` | card_id | column_id, column_name, board_id, position |
| `delete_card` | card_id | — |
| `create_column` | name | board_id, color, wip_limit |
| `manage_tags` | action | board_id, tag_id, card_id, name, color |
| `manage_phases` | action | board_id, phase_id, name, color, ordered_ids |

Column resolution: by `column_id` first, then `column_name` (case-insensitive), then defaults to first column on the board.

Board resolution: if `board_id` is empty, uses the first board by `created_at`.

Error prefixes: `VALIDATION:`, `NOT_FOUND:`, `CONFLICT:` — all returned as `mcp.NewToolResultError`.

## Testing

- 41 tests total: 6 in `internal/db`, 35 in `internal/services`
- All tests use in-memory SQLite (`:memory:`) — no external dependencies
- `newTestDB(t)` helper creates a fresh DB with migrations + seed per test
- Run: `go test ./...`

## Gotchas

- **Hook output is plain text** — Claude Code does NOT render markdown in hook `systemMessage`. Use CAPS and indentation for visual hierarchy, never `##`, `**`, or emojis.
- **Hooks don't support `env` field** — Unlike `.mcp.json`, `hooks.json` has no `env` field. Inline `${VAR}` substitution is also broken for SessionStart hooks (known Claude Code bugs). This is why `CLAUDE_PLUGIN_DATA` is excluded from DB path resolution — both MCP server and hooks must converge on `~/.cyber-mango/kanban.db`.
- **Version lives in 3 places** — `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, and `internal/mcp/server.go` (`NewMCPServer("cyber-mango", "0.2.0", ...)`). Keep them in sync on version bumps.
- **`.mcp.json` shows "Failed to connect" inside the plugin source dir** — `${CLAUDE_PLUGIN_ROOT}` isn't set when working inside the plugin repo itself. This is expected. The plugin entry works from any other directory.
- **Double slash in resolved path** — `source: "./"` in marketplace.json can produce `C:/path//bin/mcp-server.exe`. Harmless.
- **Shared DB with web UI** — The plugin and the [cyber-mango web UI](https://github.com/juandagalo/cyber-mango) share the same SQLite database. Changes from either side appear instantly.
- **Position is REAL** — Cards use `maxPos + 1`, columns use `maxPos + 1000`. Fractional positioning is supported for reordering.

## Conventions

- All IDs are 12-char nanoid (via `go-nanoid/v2`)
- All timestamps are UTC RFC3339 strings
- Every write operation (create/update/move/delete card, create column) logs to `activity_log`
- Services are stateless functions taking `*sqlx.DB` — no service structs, no interfaces
- Handlers struct (`internal/mcp/handlers.go`) holds `*sqlx.DB`, dispatches to service functions
- Error handling: hooks exit silently on error (exit 0), MCP server exits with error (exit 1)
- JSON responses: all slice fields initialized to empty `[]` (never nil) to avoid `null` in JSON

## Install as Plugin

```bash
claude plugin marketplace add /path/to/cyber-mango-plugin-go
claude plugin install cyber-mango
```

Verify: `claude mcp list` should show `plugin:cyber-mango:cyber-mango — Connected`
