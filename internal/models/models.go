package models

type Board struct {
	ID          string  `db:"id"          json:"id"`
	Name        string  `db:"name"        json:"name"`
	Description *string `db:"description" json:"description"`
	CreatedAt   string  `db:"created_at"  json:"created_at"`
	UpdatedAt   string  `db:"updated_at"  json:"updated_at"`
	// Loaded only by get_board; omitempty so list_boards never shows an empty list for a populated board.
	Columns []Column `db:"-" json:"columns,omitempty"`
	Phases  []Phase  `db:"-" json:"phases,omitempty"`
}

type Column struct {
	ID          string  `db:"id"          json:"id"`
	BoardID     string  `db:"board_id"    json:"board_id"`
	Name        string  `db:"name"        json:"name"`
	Color       string  `db:"color"       json:"color"`
	Description *string `db:"description" json:"description"`
	WipLimit    *int    `db:"wip_limit"   json:"wip_limit"`
	Position    float64 `db:"position"    json:"position"`
	CreatedAt   string  `db:"created_at"  json:"created_at"`
	UpdatedAt   string  `db:"updated_at"  json:"updated_at"`
	Cards       []Card  `db:"-" json:"cards"`
}

type Card struct {
	ID           string  `db:"id"             json:"id"`
	ColumnID     string  `db:"column_id"      json:"column_id"`
	Title        string  `db:"title"          json:"title"`
	Description  string  `db:"description"    json:"description"`
	Priority     string  `db:"priority"       json:"priority"`
	Position     float64 `db:"position"       json:"position"`
	ParentCardID *string `db:"parent_card_id" json:"parent_card_id"`
	DueDate      *string `db:"due_date"       json:"due_date"`
	PhaseID      *string `db:"phase_id"       json:"phase_id"`
	CreatedAt    string  `db:"created_at"     json:"created_at"`
	UpdatedAt    string  `db:"updated_at"     json:"updated_at"`
	Tags         []Tag   `db:"-" json:"tags"`
	Phase        *Phase  `db:"-" json:"phase,omitempty"`
}

type Phase struct {
	ID        string  `db:"id"         json:"id"`
	BoardID   string  `db:"board_id"   json:"board_id"`
	Name      string  `db:"name"       json:"name"`
	Color     string  `db:"color"      json:"color"`
	Position  float64 `db:"position"   json:"position"`
	CreatedAt string  `db:"created_at" json:"created_at"`
	UpdatedAt string  `db:"updated_at" json:"updated_at"`
}

type Tag struct {
	ID        string `db:"id"         json:"id"`
	BoardID   string `db:"board_id"   json:"board_id"`
	Name      string `db:"name"       json:"name"`
	Color     string `db:"color"      json:"color"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

type ActivityLog struct {
	ID        string  `db:"id"         json:"id"`
	BoardID   string  `db:"board_id"   json:"board_id"`
	CardID    *string `db:"card_id"    json:"card_id"`
	Action    string  `db:"action"     json:"action"`
	Details   *string `db:"details"    json:"details"`
	Agent     *string `db:"agent"      json:"agent"`
	CreatedAt string  `db:"created_at" json:"created_at"`
}

// BoardSummary is the response from get_board_summary.
type BoardSummary struct {
	BoardID    string          `json:"board_id"`
	BoardName  string          `json:"board_name"`
	TotalCards int             `json:"total_cards"`
	Columns    []ColumnSummary `json:"columns"`
	ByPriority map[string]int  `json:"by_priority"`
	ByPhase    map[string]int  `json:"by_phase"`
}

type ColumnSummary struct {
	ColumnID    string  `json:"column_id"`
	ColumnName  string  `json:"column_name"`
	Description *string `json:"description"`
	CardCount   int     `json:"card_count"`
	WipLimit    *int    `json:"wip_limit"`
}
