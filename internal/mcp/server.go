package mcp

import (
	"github.com/jmoiron/sqlx"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// NewServer creates and configures the MCP server with all 10 tools registered.
func NewServer(db *sqlx.DB) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer("cyber-mango", "0.2.0",
		mcpserver.WithToolCapabilities(true),
	)
	h := NewHandlers(db)

	// 1. list_boards
	s.AddTool(mcp.NewTool("list_boards",
		mcp.WithDescription("List all kanban boards"),
	), h.ListBoards)

	// 2. get_board
	s.AddTool(mcp.NewTool("get_board",
		mcp.WithDescription("Get a board with all columns, cards (tickets/tasks), and phases"),
		mcp.WithString("board_id", mcp.Description("Board ID (optional, defaults to first board)")),
	), h.GetBoard)

	// 3. get_board_summary
	s.AddTool(mcp.NewTool("get_board_summary",
		mcp.WithDescription("Get a board summary with card/ticket counts by column, priority, and phase"),
		mcp.WithString("board_id", mcp.Description("Board ID (optional)")),
	), h.GetBoardSummary)

	// 4. create_card
	s.AddTool(mcp.NewTool("create_card",
		mcp.WithDescription("Create a new card (ticket/task) on the kanban board"),
		mcp.WithString("title", mcp.Required(), mcp.Description("Card title. Format: [type] short imperative description. Types: feat, bug, chore, spike, docs. Example: [feat] add OAuth2 login flow")),
		mcp.WithString("column_id", mcp.Description("Column ID")),
		mcp.WithString("column_name", mcp.Description("Column name (case-insensitive)")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithString("description", mcp.Description("Card description with three sections: ## What (one sentence), ## Why (motivation), ## Context (files, services, endpoints). Do not add extra sections.")),
		mcp.WithString("priority", mcp.Description("Priority: low, medium, high, critical")),
		mcp.WithString("tags", mcp.Description("Comma-separated tag names to auto-create and assign")),
		mcp.WithString("phase_id", mcp.Description("Phase ID")),
		mcp.WithString("phase_name", mcp.Description("Phase name (case-insensitive)")),
	), h.CreateCard)

	// 5. update_card
	s.AddTool(mcp.NewTool("update_card",
		mcp.WithDescription("Update a card (ticket/task) and optionally move it to a different column. Changes metadata (title, description, priority, phase) and/or column in a single call. To only reposition within a column, use move_card instead."),
		mcp.WithString("card_id", mcp.Required(), mcp.Description("Card ID")),
		mcp.WithString("title", mcp.Description("New title. Format: [type] short imperative description. Types: feat, bug, chore, spike, docs")),
		mcp.WithString("description", mcp.Description("New description with three sections: ## What, ## Why, ## Context")),
		mcp.WithString("priority", mcp.Description("New priority: low, medium, high, critical")),
		mcp.WithString("phase_id", mcp.Description("Phase ID")),
		mcp.WithString("phase_name", mcp.Description("Phase name (case-insensitive)")),
		mcp.WithBoolean("unset_phase", mcp.Description("Set to true to remove phase from card")),
		mcp.WithString("column_id", mcp.Description("Target column ID to move the card/ticket to")),
		mcp.WithString("column_name", mcp.Description("Target column name to move the card/ticket to (case-insensitive)")),
		mcp.WithString("board_id", mcp.Description("Board ID (needed for column resolution when moving)")),
	), h.UpdateCard)

	// 6. move_card
	s.AddTool(mcp.NewTool("move_card",
		mcp.WithDescription("Move a card (ticket/task) to a different column or reposition within a column. Prefer update_card if you also need to change title, description, priority, or phase."),
		mcp.WithString("card_id", mcp.Required(), mcp.Description("Card ID")),
		mcp.WithString("column_id", mcp.Description("Target column ID")),
		mcp.WithString("column_name", mcp.Description("Target column name")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithNumber("position", mcp.Description("Target position (fractional)")),
	), h.MoveCard)

	// 7. delete_card
	s.AddTool(mcp.NewTool("delete_card",
		mcp.WithDescription("Delete a card (ticket/task) from the board"),
		mcp.WithString("card_id", mcp.Required(), mcp.Description("Card ID")),
	), h.DeleteCard)

	// 8. create_column
	s.AddTool(mcp.NewTool("create_column",
		mcp.WithDescription("Create a new column on a kanban board"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Column name")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithString("color", mcp.Description("Hex color (e.g. #3b82f6)")),
		mcp.WithString("description", mcp.Description("Column purpose — describes what this column means in the workflow. Agents use this to understand where to move cards.")),
		mcp.WithNumber("wip_limit", mcp.Description("WIP limit (optional)")),
	), h.CreateColumn)

	// 9. manage_phases
	s.AddTool(mcp.NewTool("manage_phases",
		mcp.WithDescription("Manage workflow phases on a board (list, create, update, delete, reorder)"),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action: list, create, update, delete, reorder")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithString("phase_id", mcp.Description("Phase ID")),
		mcp.WithString("name", mcp.Description("Phase name")),
		mcp.WithString("color", mcp.Description("Hex color (e.g. #00FFFF)")),
		mcp.WithString("ordered_ids", mcp.Description("JSON array of phase IDs for reorder (e.g. [\"id1\",\"id2\"])")),
	), h.ManagePhases)

	// 10. manage_tags
	s.AddTool(mcp.NewTool("manage_tags",
		mcp.WithDescription("Manage tags for cards/tickets (create, assign, remove, list, delete)"),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action: create, assign, remove, list, delete")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithString("tag_id", mcp.Description("Tag ID")),
		mcp.WithString("card_id", mcp.Description("Card ID")),
		mcp.WithString("name", mcp.Description("Tag name")),
		mcp.WithString("color", mcp.Description("Hex color")),
	), h.ManageTags)

	return s
}
