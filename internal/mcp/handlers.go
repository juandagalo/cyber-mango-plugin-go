package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/services"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handlers holds the DB connection for all tool handlers.
type Handlers struct {
	db *sqlx.DB
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(db *sqlx.DB) *Handlers {
	return &Handlers{db: db}
}

func errResult(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(msg)
}

func jsonResult(v interface{}) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errResult(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// ListBoards handles the list_boards tool.
func (h *Handlers) ListBoards(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	boards, err := services.ListBoards(h.db)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(boards)
}

// GetBoard handles the get_board tool.
func (h *Handlers) GetBoard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	boardID := req.GetString("board_id", "")
	board, err := services.GetBoard(h.db, boardID)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(board)
}

// GetBoardSummary handles the get_board_summary tool.
func (h *Handlers) GetBoardSummary(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	boardID := req.GetString("board_id", "")
	summary, err := services.GetBoardSummary(h.db, boardID)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(summary)
}

// CreateCard handles the create_card tool.
func (h *Handlers) CreateCard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title := req.GetString("title", "")
	if title == "" {
		return errResult("VALIDATION: title is required"), nil
	}
	card, err := services.CreateCard(
		h.db,
		req.GetString("board_id", ""),
		req.GetString("column_id", ""),
		req.GetString("column_name", ""),
		title,
		req.GetString("description", ""),
		req.GetString("priority", ""),
		req.GetString("tags", ""),
		req.GetString("phase_id", ""),
		req.GetString("phase_name", ""),
	)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(card)
}

// UpdateCard handles the update_card tool.
func (h *Handlers) UpdateCard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cardID := req.GetString("card_id", "")
	if cardID == "" {
		return errResult("VALIDATION: card_id is required"), nil
	}
	var unsetPhase bool
	if args := req.GetArguments(); args != nil {
		if v, ok := args["unset_phase"]; ok {
			if b, ok := v.(bool); ok {
				unsetPhase = b
			}
		}
	}

	card, err := services.UpdateCard(
		h.db, cardID,
		req.GetString("title", ""),
		req.GetString("description", ""),
		req.GetString("priority", ""),
		req.GetString("phase_id", ""),
		req.GetString("phase_name", ""),
		unsetPhase,
		req.GetString("board_id", ""),
		req.GetString("column_id", ""),
		req.GetString("column_name", ""),
	)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(card)
}

// MoveCard handles the move_card tool.
func (h *Handlers) MoveCard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cardID := req.GetString("card_id", "")
	if cardID == "" {
		return errResult("VALIDATION: card_id is required"), nil
	}

	var position *float64
	args := req.GetArguments()
	if args != nil {
		if v, ok := args["position"]; ok {
			switch f := v.(type) {
			case float64:
				position = &f
			case int:
				x := float64(f)
				position = &x
			}
		}
	}

	card, err := services.MoveCard(
		h.db, cardID,
		req.GetString("board_id", ""),
		req.GetString("column_id", ""),
		req.GetString("column_name", ""),
		position,
	)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(card)
}

// DeleteCard handles the delete_card tool.
func (h *Handlers) DeleteCard(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cardID := req.GetString("card_id", "")
	if cardID == "" {
		return errResult("VALIDATION: card_id is required"), nil
	}
	if err := services.DeleteCard(h.db, cardID); err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]interface{}{"deleted": true, "card_id": cardID})
}

// CreateColumn handles the create_column tool.
func (h *Handlers) CreateColumn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	if name == "" {
		return errResult("VALIDATION: name is required"), nil
	}

	var wipLimit *int
	args := req.GetArguments()
	if args != nil {
		if v, ok := args["wip_limit"]; ok {
			switch f := v.(type) {
			case float64:
				x := int(f)
				wipLimit = &x
			case int:
				wipLimit = &f
			}
		}
	}

	col, err := services.CreateColumn(
		h.db,
		req.GetString("board_id", ""),
		name,
		req.GetString("color", ""),
		req.GetString("description", ""),
		wipLimit,
	)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(col)
}

// ManagePhases handles the manage_phases tool.
func (h *Handlers) ManagePhases(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := req.GetString("action", "")
	if action == "" {
		return errResult("VALIDATION: action is required"), nil
	}

	var orderedIDs []string
	if action == "reorder" {
		raw := req.GetString("ordered_ids", "")
		parsed, err := services.ParseOrderedIDs(raw)
		if err != nil {
			return errResult(err.Error()), nil
		}
		orderedIDs = parsed
	}

	result, err := services.ManagePhases(
		h.db, action,
		req.GetString("board_id", ""),
		req.GetString("phase_id", ""),
		req.GetString("name", ""),
		req.GetString("color", ""),
		orderedIDs,
	)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(result)
}

// ManageTags handles the manage_tags tool.
func (h *Handlers) ManageTags(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := req.GetString("action", "")
	if action == "" {
		return errResult("VALIDATION: action is required"), nil
	}

	result, err := services.ManageTags(
		h.db, action,
		req.GetString("board_id", ""),
		req.GetString("tag_id", ""),
		req.GetString("card_id", ""),
		req.GetString("name", ""),
		req.GetString("color", ""),
	)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(result)
}
