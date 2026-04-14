package mcp

import (
	"github.com/jmoiron/sqlx"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// NewServer creates and configures the MCP server with all 9 tools registered.
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
		mcp.WithDescription("Get a board with all its columns and cards"),
		mcp.WithString("board_id", mcp.Description("Board ID (optional, defaults to first board)")),
	), h.GetBoard)

	// 3. get_board_summary
	s.AddTool(mcp.NewTool("get_board_summary",
		mcp.WithDescription("Get a summary of a board (card counts by column and priority)"),
		mcp.WithString("board_id", mcp.Description("Board ID (optional)")),
	), h.GetBoardSummary)

	// 4. create_card
	s.AddTool(mcp.NewTool("create_card",
		mcp.WithDescription("Create a new card on the kanban board"),
		mcp.WithString("title", mcp.Required(), mcp.Description("Card title")),
		mcp.WithString("column_id", mcp.Description("Column ID")),
		mcp.WithString("column_name", mcp.Description("Column name (case-insensitive)")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithString("description", mcp.Description("Card description")),
		mcp.WithString("priority", mcp.Description("Priority: low, medium, high, critical")),
		mcp.WithString("tags", mcp.Description("Comma-separated tag names to auto-create and assign")),
	), h.CreateCard)

	// 5. update_card
	s.AddTool(mcp.NewTool("update_card",
		mcp.WithDescription("Update a card title, description, or priority"),
		mcp.WithString("card_id", mcp.Required(), mcp.Description("Card ID")),
		mcp.WithString("title", mcp.Description("New title")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("priority", mcp.Description("New priority: low, medium, high, critical")),
	), h.UpdateCard)

	// 6. move_card
	s.AddTool(mcp.NewTool("move_card",
		mcp.WithDescription("Move a card to a different column or position"),
		mcp.WithString("card_id", mcp.Required(), mcp.Description("Card ID")),
		mcp.WithString("column_id", mcp.Description("Target column ID")),
		mcp.WithString("column_name", mcp.Description("Target column name")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithNumber("position", mcp.Description("Target position (fractional)")),
	), h.MoveCard)

	// 7. delete_card
	s.AddTool(mcp.NewTool("delete_card",
		mcp.WithDescription("Delete a card from the board"),
		mcp.WithString("card_id", mcp.Required(), mcp.Description("Card ID")),
	), h.DeleteCard)

	// 8. create_column
	s.AddTool(mcp.NewTool("create_column",
		mcp.WithDescription("Create a new column on a kanban board"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Column name")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithString("color", mcp.Description("Hex color (e.g. #3b82f6)")),
		mcp.WithNumber("wip_limit", mcp.Description("WIP limit (optional)")),
	), h.CreateColumn)

	// 9. manage_tags
	s.AddTool(mcp.NewTool("manage_tags",
		mcp.WithDescription("Create, assign, remove, list, or delete tags"),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action: create, assign, remove, list, delete")),
		mcp.WithString("board_id", mcp.Description("Board ID")),
		mcp.WithString("tag_id", mcp.Description("Tag ID")),
		mcp.WithString("card_id", mcp.Description("Card ID")),
		mcp.WithString("name", mcp.Description("Tag name")),
		mcp.WithString("color", mcp.Description("Hex color")),
	), h.ManageTags)

	return s
}
