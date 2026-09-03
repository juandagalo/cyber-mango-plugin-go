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
make build        # produces bin/mcp-server.exe, bin/session-start.exe, bin/session-stop.exe (.exe on every OS)
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
| `sqltx` | `sqltx.go`, `sqltx_test.go` | `Run(db, fn)`: begin/commit, rollback on error (returned unchanged) or panic. |
| `mcp` | `server.go`, `handlers.go` | MCP tool registration and handler dispatch. `Handlers` struct holds `*sqlx.DB`. Uses `req.GetString(key, "")` (mcp-go v0.44.0 API). |

### Plugin Metadata (.claude-plugin/)

| File | Purpose |
|------|---------|
| `plugin.json` | Plugin identity: name, version, description, author. |
| `marketplace.json` | Self-contained marketplace. `source: "./"` makes the plugin its own marketplace. Root schema: only `name`, `owner`, `plugins` (no description/version at root). |

### MCP Config (.mcp.json)

Uses `${CLAUDE_PLUGIN_ROOT}` to resolve binary paths. Passes no env vars: the DB path resolves inside the binary (see Database section).

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
- `columns` — id, board_id (FK), name, color, description TEXT, wip_limit, position (REAL), timestamps
- `phases` — id, board_id (FK), name, color, position (REAL), timestamps. Unique index on (board_id, name).
- `cards` — id, column_id (FK), title, description, priority (CHECK: low/medium/high/critical), position (REAL), parent_card_id, due_date, phase_id (FK nullable, ON DELETE SET NULL), timestamps
- `tags` — id, board_id (FK), name, color. Unique index on (board_id, name).
- `card_tags` — card_id + tag_id (composite PK, both FK with CASCADE)
- `activity_log` — id, board_id (FK), card_id, action, details, agent, timestamp
- `_meta` — key/value for schema versioning (current: "3")
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
| `update_card` | card_id | title, description, priority, phase_id, phase_name, unset_phase, column_id, column_name, board_id |
| `move_card` | card_id | column_id, column_name, board_id, position |
| `delete_card` | card_id | — |
| `create_column` | name | board_id, color, wip_limit, description |
| `manage_tags` | action | board_id, tag_id, card_id, name, color |
| `manage_phases` | action | board_id, phase_id, name, color, ordered_ids |

Column resolution: by `column_id` first, then `column_name` (case-insensitive), then defaults to first column on the board.

Board resolution: if `board_id` is empty, uses the first board by `created_at`.

Error prefixes: `VALIDATION:`, `NOT_FOUND:`, `CONFLICT:` — all returned as `mcp.NewToolResultError`. `NOT_FOUND:` is produced only from `sql.ErrNoRows`; any other DB error is wrapped with `%w` so its SQLite cause survives.

## Testing

- 79 tests total: 14 in `internal/db`, 62 in `internal/services`, 3 in `internal/sqltx`
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
- **Migration chaining** — `RunMigrations` must update its local `version` variable after each step, otherwise a v1 DB stops at v2 in a single run. Keep this pattern when adding v4+.
- **Drizzle journal formats coexist** — The web UI writes SHA-256 content hashes into `__drizzle_migrations`; the Go plugin writes tag names (`0001_right_polaris`, `0002_old_vengeance`, `0003_overjoyed_reaper`). Each side checks its own format, so both rows can live in the same table. When the web UI adds a migration, add the matching tag here.

## Conventions

- All IDs are 12-char nanoid (via `go-nanoid/v2`)
- All timestamps are UTC RFC3339 strings
- Every write operation (card create/update/move/delete, column create, phase create/update/delete/reorder, tag create/assign/remove/delete) logs to `activity_log`, and a `LogActivity` error propagates to the caller
- Services are stateless functions taking `*sqlx.DB` — no service structs. The one interface is `services.Querier` (satisfied by `*sqlx.DB` and `*sqlx.Tx`), used only by helpers that must run inside a transaction
- Multi-statement writes (create card + tags, reorder phases, seed, each migration step) run inside `sqltx.Run`. Everything inside the closure MUST use the `*sqlx.Tx`: the pool has one connection, so a call on `*sqlx.DB` while a tx is open blocks forever
- Handlers struct (`internal/mcp/handlers.go`) holds `*sqlx.DB`, dispatches to service functions
- Error handling: hooks exit silently on error (exit 0), MCP server exits with error (exit 1)
- JSON responses: `Column.Cards`, `Card.Tags`, and every list result are always `[]`, never `null` or omitted. `Board.Columns` and `Board.Phases` are the one exception: they carry `omitempty` because only `get_board` loads them and `list_boards` must not show an empty list for a populated board
- Column descriptions are used by agents to understand workflow dynamically — agents call `get_board` and read each column's `description` field instead of relying on hardcoded column names

## Status and Pending Work

Delivered features, in order: Go rewrite (v0.1.0), card phases, `update_card` column move, card template convention, column descriptions (schema v3, v0.2.0).

Known pending items:

- `update_column` tool does not exist. Columns created before schema v3 have `NULL` description and there is no way to fill them from an agent. A card for this exists in the board's Backlog.
- End-to-end checks not yet recorded: hooks output verified from a directory outside the plugin repo, and shared-DB round trip against the web UI.
- No `.gitattributes`. With `core.autocrlf=true` git warns "LF will be replaced by CRLF" on every touched file and `gofmt -l` flags CRLF files. Add `* text=auto eol=lf` and one normalization commit.

### Hardening pass (started 2026-09-03)

Audit findings being fixed with TDD, in this order. Mark each `[x]` when its test is green and the change is committed.

- [x] H1 Pragmas per connection: apply via DSN `_pragma=` and `SetMaxOpenConns(1)` (`internal/db/connection.go`)
- [x] H2 `move_card` with only `position` must keep the current column (`internal/services/card_service.go`)
- [x] H3 Drop `omitempty` on `Column.Cards` and `Card.Tags`; `ListBoards` returns `[]` not `null`. `Board.Columns`/`Phases` keep `omitempty` on purpose: only `get_board` loads them
- [x] H4 Transactions around create card + tags, reorder phases, seed, migration (`internal/sqltx`, `services.Querier`)
- [x] H5 Tag writes log activity (`tag_created/assigned/removed/deleted`, each inside `sqltx.Run`); `LogActivity` errors propagate everywhere
- [x] H6 Only `sql.ErrNoRows` maps to `NOT_FOUND`; other DB errors keep their cause (`getCard`, `getPhase`, `getTag` helpers)
- [ ] H7 Migration runs the `pragma_table_info` guards even on fresh-version stamp (Drizzle-first DBs)
- [ ] H8 `session-stop` watermark per session, not global: read `session_id` from the hook stdin JSON, store one watermark per session, and compare with `>=` plus dedupe by ID (or RFC3339Nano timestamps) so same-second activity is not dropped. Concurrent sessions on the shared DB must not steal each other's summaries
- [ ] H9 `GetBoard` batched queries (no N+1); `session-start` calls it once
- [ ] H10 `GetBoardSummary` single GROUP BY query
- [ ] H11 Indexes on `activity_log`; `COLLATE NOCASE` name lookups for tags/phases
- [ ] H12 Deduplicate column SELECT, priority/color validation, first-board resolution
- [ ] H13 `session-start` phase list sorted by position, not map order

Update this section when a feature ships or a pending item closes.

## Install as Plugin

```bash
claude plugin marketplace add /path/to/cyber-mango-plugin-go
claude plugin install cyber-mango
```

Verify: `claude mcp list` should show `plugin:cyber-mango:cyber-mango — Connected`
