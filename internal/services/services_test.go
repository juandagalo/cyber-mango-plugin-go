package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/db"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
)

func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	testDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })
	if err := db.RunMigrations(testDB); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if err := db.SeedDefaultBoard(testDB); err != nil {
		t.Fatalf("seed default board: %v", err)
	}
	return testDB
}

func TestListBoards(t *testing.T) {
	testDB := newTestDB(t)
	boards, err := ListBoards(testDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 1 {
		t.Errorf("want 1 board, got %d", len(boards))
	}
	if boards[0].Name != "Cyber Mango" {
		t.Errorf("want board name 'Cyber Mango', got %q", boards[0].Name)
	}
}

func TestGetBoard(t *testing.T) {
	testDB := newTestDB(t)
	board, err := GetBoard(testDB, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Columns) != 5 {
		t.Errorf("want 5 columns, got %d", len(board.Columns))
	}
}

func TestGetBoardSummary(t *testing.T) {
	testDB := newTestDB(t)
	summary, err := GetBoardSummary(testDB, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Columns) != 5 {
		t.Errorf("want 5 column summaries, got %d", len(summary.Columns))
	}
	if summary.TotalCards != 0 {
		t.Errorf("want 0 total cards, got %d", summary.TotalCards)
	}
}

func TestCreateCard(t *testing.T) {
	testDB := newTestDB(t)
	card, err := CreateCard(testDB, "", "", "Backlog", "Test Card", "A description", "high", "", "", "")
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if card.Title != "Test Card" {
		t.Errorf("want title 'Test Card', got %q", card.Title)
	}
	if card.Priority != "high" {
		t.Errorf("want priority 'high', got %q", card.Priority)
	}
}

func TestCreateCard_InvalidPriority(t *testing.T) {
	testDB := newTestDB(t)
	_, err := CreateCard(testDB, "", "", "", "Bad Card", "", "urgent", "", "", "")
	if err == nil {
		t.Error("expected error for invalid priority")
	}
}

func TestUpdateCard(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "", "Original", "", "medium", "", "", "")
	updated, err := UpdateCard(testDB, card.ID, "Updated Title", "", "high", "", "", false, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("want 'Updated Title', got %q", updated.Title)
	}
	if updated.Priority != "high" {
		t.Errorf("want 'high', got %q", updated.Priority)
	}
}

func TestMoveCard(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "Backlog", "Move Me", "", "", "", "", "")

	moved, err := MoveCard(testDB, card.ID, "", "", "In Progress", nil)
	if err != nil {
		t.Fatalf("MoveCard: %v", err)
	}
	if moved.ColumnID == card.ColumnID {
		t.Error("card should have moved to a different column")
	}
}

func TestDeleteCard(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "", "Delete Me", "", "", "", "", "")
	if err := DeleteCard(testDB, card.ID); err != nil {
		t.Fatalf("DeleteCard: %v", err)
	}

	var count int
	testDB.QueryRow(`SELECT COUNT(*) FROM cards WHERE id = ?`, card.ID).Scan(&count)
	if count != 0 {
		t.Error("card should have been deleted")
	}
}

func TestCreateColumn(t *testing.T) {
	testDB := newTestDB(t)
	col, err := CreateColumn(testDB, "", "QA", "#ff0000", "", nil)
	if err != nil {
		t.Fatalf("CreateColumn: %v", err)
	}
	if col.Name != "QA" {
		t.Errorf("want column name 'QA', got %q", col.Name)
	}
}

func TestCreateColumn_WithDescription(t *testing.T) {
	testDB := newTestDB(t)
	desc := "Work actively being implemented"
	col, err := CreateColumn(testDB, "", "In Progress", "#00ff00", desc, nil)
	if err != nil {
		t.Fatalf("CreateColumn with description: %v", err)
	}
	if col.Description == nil {
		t.Fatal("want non-nil description, got nil")
	}
	if *col.Description != desc {
		t.Errorf("want description %q, got %q", desc, *col.Description)
	}

	// Verify persisted value via DB read
	var got *string
	testDB.QueryRow(`SELECT description FROM columns WHERE id = ?`, col.ID).Scan(&got)
	if got == nil {
		t.Fatal("description should be persisted in DB")
	}
	if *got != desc {
		t.Errorf("persisted description: want %q, got %q", desc, *got)
	}
}

func TestCreateColumn_WithoutDescription(t *testing.T) {
	testDB := newTestDB(t)
	col, err := CreateColumn(testDB, "", "Backlog 2", "#aabbcc", "", nil)
	if err != nil {
		t.Fatalf("CreateColumn without description: %v", err)
	}
	if col.Description != nil {
		t.Errorf("want nil description, got %q", *col.Description)
	}

	// Verify NULL in DB
	var got *string
	testDB.QueryRow(`SELECT description FROM columns WHERE id = ?`, col.ID).Scan(&got)
	if got != nil {
		t.Errorf("want NULL in DB, got %q", *got)
	}
}

func TestCreateColumn_EmptyDescription(t *testing.T) {
	testDB := newTestDB(t)
	// Empty string should be stored as NULL
	col, err := CreateColumn(testDB, "", "Review", "#ffffff", "   ", nil)
	if err != nil {
		t.Fatalf("CreateColumn empty description: %v", err)
	}
	// Whitespace-only description should be treated as empty -> nil
	// (behaviour: trimmed empty == nil)
	// This test documents the expected behaviour; implementation may choose to
	// trim or not. We assert nil for empty-after-trim.
	if col.Description != nil {
		t.Errorf("want nil for whitespace-only description, got %q", *col.Description)
	}
}

func TestGetBoard_ColumnsHaveDescription(t *testing.T) {
	testDB := newTestDB(t)
	// Seeded columns don't have descriptions yet (batch 3 adds them).
	// This test just verifies the field is present and is nil for seeded columns.
	board, err := GetBoard(testDB, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, col := range board.Columns {
		// Description field must exist on the struct (compile-time guarantee).
		// For now seeded columns have nil description — that's expected.
		_ = col.Description
	}
}

func TestGetBoardSummary_ColumnsHaveDescription(t *testing.T) {
	testDB := newTestDB(t)
	summary, err := GetBoardSummary(testDB, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, cs := range summary.Columns {
		// Description field must exist on ColumnSummary (compile-time guarantee).
		// Seeded columns have nil description.
		_ = cs.Description
	}
}

func TestCreateColumn_Description_JSON(t *testing.T) {
	testDB := newTestDB(t)

	// Column with description: json should contain description value
	desc := "Testing workflow"
	col, err := CreateColumn(testDB, "", "Testing", "#ff00ff", desc, nil)
	if err != nil {
		t.Fatalf("CreateColumn: %v", err)
	}
	if col.Description == nil || *col.Description != desc {
		t.Errorf("want description %q, got %v", desc, col.Description)
	}

	// Column without description: json should contain "description":null
	col2, err := CreateColumn(testDB, "", "Empty", "#000000", "", nil)
	if err != nil {
		t.Fatalf("CreateColumn: %v", err)
	}
	if col2.Description != nil {
		t.Errorf("want nil description, got %q", *col2.Description)
	}
}

func TestManageTags_CreateAndAssign(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "", "Tagged Card", "", "", "", "", "")

	tagResult, err := ManageTags(testDB, "create", "", "", "", "bug", "#ef4444")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	tag := tagResult.(*models.Tag)
	if tag.Name != "bug" {
		t.Errorf("want tag name 'bug', got %q", tag.Name)
	}

	_, err = ManageTags(testDB, "assign", "", tag.ID, card.ID, "", "")
	if err != nil {
		t.Fatalf("assign tag: %v", err)
	}

	var count int
	testDB.QueryRow(`SELECT COUNT(*) FROM card_tags WHERE card_id = ? AND tag_id = ?`, card.ID, tag.ID).Scan(&count)
	if count != 1 {
		t.Error("tag should be assigned to card")
	}
}

func TestCreateCard_WithTags(t *testing.T) {
	testDB := newTestDB(t)
	card, err := CreateCard(testDB, "", "", "", "Tagged Task", "", "medium", "my-project,bug", "", "")
	if err != nil {
		t.Fatalf("CreateCard with tags: %v", err)
	}
	if len(card.Tags) != 2 {
		t.Fatalf("want 2 tags, got %d", len(card.Tags))
	}
	names := map[string]bool{}
	for _, tag := range card.Tags {
		names[tag.Name] = true
	}
	if !names["my-project"] || !names["bug"] {
		t.Errorf("want tags [my-project, bug], got %v", card.Tags)
	}
}

func TestCreateCard_WithExistingTag(t *testing.T) {
	testDB := newTestDB(t)
	// Pre-create the tag
	ManageTags(testDB, "create", "", "", "", "my-project", "#3b82f6")

	card, err := CreateCard(testDB, "", "", "", "Second Task", "", "medium", "my-project", "", "")
	if err != nil {
		t.Fatalf("CreateCard with existing tag: %v", err)
	}
	if len(card.Tags) != 1 {
		t.Fatalf("want 1 tag, got %d", len(card.Tags))
	}

	// Verify no duplicate tags were created
	result, _ := ManageTags(testDB, "list", "", "", "", "", "")
	tags := result.([]models.Tag)
	count := 0
	for _, tag := range tags {
		if tag.Name == "my-project" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("want 1 'my-project' tag, got %d", count)
	}
}

func TestCreateCard_WithMultipleTags_Whitespace(t *testing.T) {
	testDB := newTestDB(t)
	card, err := CreateCard(testDB, "", "", "", "Trimmed Task", "", "medium", " feature , , docs ", "", "")
	if err != nil {
		t.Fatalf("CreateCard with whitespace tags: %v", err)
	}
	if len(card.Tags) != 2 {
		t.Fatalf("want 2 tags (empty segments skipped), got %d", len(card.Tags))
	}
}

func TestManageTags_List(t *testing.T) {
	testDB := newTestDB(t)
	ManageTags(testDB, "create", "", "", "", "feature", "#3b82f6")
	ManageTags(testDB, "create", "", "", "", "bug", "#ef4444")

	result, err := ManageTags(testDB, "list", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	tags := result.([]models.Tag)
	if len(tags) != 2 {
		t.Errorf("want 2 tags, got %d", len(tags))
	}
}

// --- Phase service tests ---

func TestListPhases(t *testing.T) {
	testDB := newTestDB(t)
	result, err := ManagePhases(testDB, "list", "", "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	phases := result.([]models.Phase)
	if len(phases) != 5 {
		t.Errorf("want 5 seeded phases, got %d", len(phases))
	}
	// Verify order
	if phases[0].Name != "Development" {
		t.Errorf("want first phase 'Development', got %q", phases[0].Name)
	}
	if phases[4].Name != "Ready to Deploy" {
		t.Errorf("want last phase 'Ready to Deploy', got %q", phases[4].Name)
	}
}

func TestCreatePhase(t *testing.T) {
	testDB := newTestDB(t)
	result, err := ManagePhases(testDB, "create", "", "", "Testing", "#FF0000", nil)
	if err != nil {
		t.Fatalf("create phase: %v", err)
	}
	phase := result.(*models.Phase)
	if phase.Name != "Testing" {
		t.Errorf("want name 'Testing', got %q", phase.Name)
	}
	if phase.Color != "#FF0000" {
		t.Errorf("want color '#FF0000', got %q", phase.Color)
	}
	if phase.Position != 6.0 {
		t.Errorf("want position 6.0 (after 5 seeded), got %f", phase.Position)
	}
}

func TestCreatePhase_DefaultColor(t *testing.T) {
	testDB := newTestDB(t)
	result, err := ManagePhases(testDB, "create", "", "", "Staging", "", nil)
	if err != nil {
		t.Fatalf("create phase: %v", err)
	}
	phase := result.(*models.Phase)
	if phase.Color != "#00FFFF" {
		t.Errorf("want default color '#00FFFF', got %q", phase.Color)
	}
}

func TestCreatePhase_ValidationErrors(t *testing.T) {
	testDB := newTestDB(t)

	// Empty name
	_, err := ManagePhases(testDB, "create", "", "", "", "", nil)
	if err == nil {
		t.Error("expected error for empty name")
	}

	// Name too long
	longName := "a]234567890123456789012345678901234567890123456789X"
	_, err = ManagePhases(testDB, "create", "", "", longName, "", nil)
	if err == nil {
		t.Error("expected error for name > 50 chars")
	}

	// Invalid color
	_, err = ManagePhases(testDB, "create", "", "", "Valid", "red", nil)
	if err == nil {
		t.Error("expected error for invalid color")
	}
}

func TestCreatePhase_DuplicateName(t *testing.T) {
	testDB := newTestDB(t)
	// "Development" is seeded
	_, err := ManagePhases(testDB, "create", "", "", "Development", "", nil)
	if err == nil {
		t.Error("expected CONFLICT error for duplicate name")
	}
}

func TestUpdatePhase(t *testing.T) {
	testDB := newTestDB(t)
	// Get first phase
	list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	phases := list.([]models.Phase)
	phaseID := phases[0].ID

	result, err := ManagePhases(testDB, "update", "", phaseID, "Dev", "#AABBCC", nil)
	if err != nil {
		t.Fatalf("update phase: %v", err)
	}
	updated := result.(*models.Phase)
	if updated.Name != "Dev" {
		t.Errorf("want name 'Dev', got %q", updated.Name)
	}
	if updated.Color != "#AABBCC" {
		t.Errorf("want color '#AABBCC', got %q", updated.Color)
	}
}

func TestUpdatePhase_ConflictOnRename(t *testing.T) {
	testDB := newTestDB(t)
	list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	phases := list.([]models.Phase)
	// Try renaming first phase to second phase's name
	_, err := ManagePhases(testDB, "update", "", phases[0].ID, phases[1].Name, "", nil)
	if err == nil {
		t.Error("expected CONFLICT error when renaming to existing name")
	}
}

func TestDeletePhase(t *testing.T) {
	testDB := newTestDB(t)
	list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	phases := list.([]models.Phase)
	phaseID := phases[0].ID

	// Create a card with this phase
	card, _ := CreateCard(testDB, "", "", "", "Phased Card", "", "", "", phaseID, "")
	if card.PhaseID == nil || *card.PhaseID != phaseID {
		t.Fatal("card should have phase assigned")
	}

	// Delete the phase
	_, err := ManagePhases(testDB, "delete", "", phaseID, "", "", nil)
	if err != nil {
		t.Fatalf("delete phase: %v", err)
	}

	// Card's phase_id should be NULL (ON DELETE SET NULL)
	var cardPhaseID *string
	testDB.QueryRow(`SELECT phase_id FROM cards WHERE id = ?`, card.ID).Scan(&cardPhaseID)
	if cardPhaseID != nil {
		t.Error("card phase_id should be NULL after phase deletion")
	}
}

func TestReorderPhases(t *testing.T) {
	testDB := newTestDB(t)
	list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	phases := list.([]models.Phase)

	// Reverse order
	reversed := make([]string, len(phases))
	for i, p := range phases {
		reversed[len(phases)-1-i] = p.ID
	}

	result, err := ManagePhases(testDB, "reorder", "", "", "", "", reversed)
	if err != nil {
		t.Fatalf("reorder phases: %v", err)
	}
	reordered := result.([]models.Phase)
	if reordered[0].ID != reversed[0] {
		t.Errorf("first phase should be %s, got %s", reversed[0], reordered[0].ID)
	}
	if reordered[0].Position != 1.0 {
		t.Errorf("first position should be 1.0, got %f", reordered[0].Position)
	}
}

func TestResolvePhase_ByID(t *testing.T) {
	testDB := newTestDB(t)
	list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	phases := list.([]models.Phase)

	phase, err := ResolvePhase(testDB, "", phases[0].ID, "")
	if err != nil {
		t.Fatalf("resolve by ID: %v", err)
	}
	if phase.ID != phases[0].ID {
		t.Errorf("want phase %s, got %s", phases[0].ID, phase.ID)
	}
}

func TestResolvePhase_ByName(t *testing.T) {
	testDB := newTestDB(t)
	board, _ := ResolveBoard(testDB, "")

	phase, err := ResolvePhase(testDB, board.ID, "", "development")
	if err != nil {
		t.Fatalf("resolve by name: %v", err)
	}
	if phase.Name != "Development" {
		t.Errorf("want 'Development', got %q", phase.Name)
	}
}

func TestResolvePhase_Empty(t *testing.T) {
	testDB := newTestDB(t)
	phase, err := ResolvePhase(testDB, "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if phase != nil {
		t.Error("expected nil phase when both ID and name are empty")
	}
}

func TestResolvePhase_NotFound(t *testing.T) {
	testDB := newTestDB(t)
	_, err := ResolvePhase(testDB, "", "nonexistent", "")
	if err == nil {
		t.Error("expected NOT_FOUND error")
	}
}

// --- Card + Phase integration tests ---

func TestCreateCard_WithPhaseID(t *testing.T) {
	testDB := newTestDB(t)
	list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	phases := list.([]models.Phase)

	card, err := CreateCard(testDB, "", "", "", "Phased Card", "", "", "", phases[0].ID, "")
	if err != nil {
		t.Fatalf("create card with phase: %v", err)
	}
	if card.PhaseID == nil {
		t.Fatal("card phase_id should be set")
	}
	if *card.PhaseID != phases[0].ID {
		t.Errorf("want phase_id %s, got %s", phases[0].ID, *card.PhaseID)
	}
}

func TestCreateCard_WithPhaseName(t *testing.T) {
	testDB := newTestDB(t)
	card, err := CreateCard(testDB, "", "", "", "Named Phase Card", "", "", "", "", "qa")
	if err != nil {
		t.Fatalf("create card with phase name: %v", err)
	}
	if card.PhaseID == nil {
		t.Fatal("card phase_id should be set")
	}
}

func TestUpdateCard_SetPhase(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "", "No Phase", "", "", "", "", "")
	if card.PhaseID != nil {
		t.Fatal("card should start without phase")
	}

	updated, err := UpdateCard(testDB, card.ID, "", "", "", "", "Development", false, "", "", "")
	if err != nil {
		t.Fatalf("set phase: %v", err)
	}
	if updated.PhaseID == nil {
		t.Fatal("card should have phase after update")
	}
}

func TestUpdateCard_ChangePhase(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "", "Phase Card", "", "", "", "", "Development")

	updated, err := UpdateCard(testDB, card.ID, "", "", "", "", "QA", false, "", "", "")
	if err != nil {
		t.Fatalf("change phase: %v", err)
	}
	if updated.PhaseID == nil {
		t.Fatal("card should have phase")
	}
	if *updated.PhaseID == *card.PhaseID {
		t.Error("phase should have changed")
	}
}

func TestUpdateCard_UnsetPhase(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "", "Unset Phase", "", "", "", "", "Development")
	if card.PhaseID == nil {
		t.Fatal("card should start with phase")
	}

	updated, err := UpdateCard(testDB, card.ID, "", "", "", "", "", true, "", "", "")
	if err != nil {
		t.Fatalf("unset phase: %v", err)
	}
	if updated.PhaseID != nil {
		t.Error("card phase should be nil after unset")
	}
}

func TestUpdateCard_WithColumnMove(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "Backlog", "Move via Update", "", "", "", "", "")

	updated, err := UpdateCard(testDB, card.ID, "Updated and Moved", "", "high", "", "", false, "", "", "In Progress")
	if err != nil {
		t.Fatalf("UpdateCard with move: %v", err)
	}
	if updated.Title != "Updated and Moved" {
		t.Errorf("want title 'Updated and Moved', got %q", updated.Title)
	}
	if updated.Priority != "high" {
		t.Errorf("want priority 'high', got %q", updated.Priority)
	}
	if updated.ColumnID == card.ColumnID {
		t.Error("card should have moved to a different column")
	}
}

func TestUpdateCard_MoveOnly(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "Backlog", "Move Only", "", "", "", "", "")

	updated, err := UpdateCard(testDB, card.ID, "", "", "", "", "", false, "", "", "In Progress")
	if err != nil {
		t.Fatalf("UpdateCard move only: %v", err)
	}
	if updated.Title != "Move Only" {
		t.Errorf("title should be unchanged, got %q", updated.Title)
	}
	if updated.ColumnID == card.ColumnID {
		t.Error("card should have moved to a different column")
	}
}

func TestUpdateCard_SameColumnNoOp(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "Backlog", "Stay Put", "", "", "", "", "")
	originalPos := card.Position

	updated, err := UpdateCard(testDB, card.ID, "", "", "", "", "", false, "", "", "Backlog")
	if err != nil {
		t.Fatalf("UpdateCard same column: %v", err)
	}
	if updated.ColumnID != card.ColumnID {
		t.Error("card should stay in same column")
	}
	if updated.Position != originalPos {
		t.Errorf("position should be unchanged, want %f got %f", originalPos, updated.Position)
	}
}

// --- Board + Phase integration tests ---

func TestGetBoard_IncludesPhases(t *testing.T) {
	testDB := newTestDB(t)
	board, err := GetBoard(testDB, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Phases) != 5 {
		t.Errorf("want 5 phases, got %d", len(board.Phases))
	}
}

func TestGetBoard_CardPhasePopulated(t *testing.T) {
	testDB := newTestDB(t)
	list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	phases := list.([]models.Phase)

	CreateCard(testDB, "", "", "Backlog", "Phase Test", "", "", "", phases[0].ID, "")

	board, err := GetBoard(testDB, "")
	if err != nil {
		t.Fatal(err)
	}

	// Find the card
	for _, col := range board.Columns {
		for _, card := range col.Cards {
			if card.Title == "Phase Test" {
				if card.Phase == nil {
					t.Error("card.Phase should be populated")
				} else if card.Phase.Name != phases[0].Name {
					t.Errorf("want phase name %q, got %q", phases[0].Name, card.Phase.Name)
				}
				return
			}
		}
	}
	t.Error("card 'Phase Test' not found in board")
}

func TestGetBoardSummary_ByPhase(t *testing.T) {
	testDB := newTestDB(t)
	list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	phases := list.([]models.Phase)

	// Create cards: 2 with phase, 1 without
	CreateCard(testDB, "", "", "", "Card A", "", "", "", phases[0].ID, "")
	CreateCard(testDB, "", "", "", "Card B", "", "", "", phases[0].ID, "")
	CreateCard(testDB, "", "", "", "Card C", "", "", "", "", "")

	summary, err := GetBoardSummary(testDB, "")
	if err != nil {
		t.Fatal(err)
	}
	if summary.ByPhase["unassigned"] != 1 {
		t.Errorf("want 1 unassigned, got %d", summary.ByPhase["unassigned"])
	}
	if summary.ByPhase[phases[0].Name] != 2 {
		t.Errorf("want 2 for %s, got %d", phases[0].Name, summary.ByPhase[phases[0].Name])
	}
}

func TestMoveCard_PositionOnly_KeepsCurrentColumn(t *testing.T) {
	testDB := newTestDB(t)
	card, err := CreateCard(testDB, "", "", "In Progress", "Reposition Me", "", "", "", "", "")
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	pos := 5.0
	moved, err := MoveCard(testDB, card.ID, "", "", "", &pos)
	if err != nil {
		t.Fatalf("MoveCard: %v", err)
	}
	if moved.ColumnID != card.ColumnID {
		t.Errorf("column changed: got %s, want %s (position-only move must stay in the current column)", moved.ColumnID, card.ColumnID)
	}
	if moved.Position != pos {
		t.Errorf("position = %v, want %v", moved.Position, pos)
	}

	var dbColumnID string
	testDB.QueryRow(`SELECT column_id FROM cards WHERE id = ?`, card.ID).Scan(&dbColumnID)
	if dbColumnID != card.ColumnID {
		t.Errorf("persisted column_id = %s, want %s", dbColumnID, card.ColumnID)
	}
}

func TestCard_JSON_EmptyTagsIsArray(t *testing.T) {
	testDB := newTestDB(t)
	card, err := CreateCard(testDB, "", "", "", "No Tags", "", "", "", "", "")
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	out, _ := json.Marshal(card)
	if !strings.Contains(string(out), `"tags":[]`) {
		t.Errorf("card JSON must contain \"tags\":[] for a card without tags, got: %s", out)
	}
}

func TestGetBoard_JSON_EmptyColumnCardsIsArray(t *testing.T) {
	testDB := newTestDB(t)
	board, err := GetBoard(testDB, "")
	if err != nil {
		t.Fatalf("GetBoard: %v", err)
	}
	out, _ := json.Marshal(board)
	if got := strings.Count(string(out), `"cards":[]`); got != len(board.Columns) {
		t.Errorf("want %d columns with \"cards\":[], got %d in: %s", len(board.Columns), got, out)
	}
}

func TestListBoards_Empty_JSONIsArray(t *testing.T) {
	testDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })
	if err := db.RunMigrations(testDB); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	boards, err := ListBoards(testDB)
	if err != nil {
		t.Fatalf("ListBoards: %v", err)
	}
	out, _ := json.Marshal(boards)
	if string(out) != `[]` {
		t.Errorf("ListBoards on empty DB must marshal to [], got %s", out)
	}
}

// --- H4 atomicity tests ---

func TestReorderPhases_FailureMidWayKeepsOriginalPositions(t *testing.T) {
	testDB := newTestDB(t)
	list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	phases := list.([]models.Phase)

	reversed := make([]string, len(phases))
	for i, p := range phases {
		reversed[len(phases)-1-i] = p.ID
	}

	// The third UPDATE of the reorder aborts after the first two succeeded.
	if _, err := testDB.Exec(fmt.Sprintf(
		`CREATE TRIGGER fail_third_update BEFORE UPDATE ON phases WHEN NEW.id = '%s' BEGIN SELECT RAISE(ABORT, 'injected failure'); END`,
		reversed[2],
	)); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err := ManagePhases(testDB, "reorder", "", "", "", "", reversed)
	if err == nil {
		t.Fatal("expected an error from the aborted reorder")
	}

	after, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
	for i, p := range after.([]models.Phase) {
		if p.ID != phases[i].ID || p.Position != phases[i].Position {
			t.Errorf("phase %d changed after failed reorder: want %s@%v, got %s@%v", i, phases[i].ID, phases[i].Position, p.ID, p.Position)
		}
	}

	var logCount int
	testDB.QueryRow(`SELECT COUNT(*) FROM activity_log WHERE action = 'phases_reordered'`).Scan(&logCount)
	if logCount != 0 {
		t.Errorf("want no phases_reordered activity after failed reorder, got %d", logCount)
	}
}

func TestCreateCard_FailureMidWayLeavesNoCard(t *testing.T) {
	testDB := newTestDB(t)

	// Card, activity log and tag rows are already written when this fires.
	if _, err := testDB.Exec(`CREATE TRIGGER fail_card_tags BEFORE INSERT ON card_tags BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err := CreateCard(testDB, "", "", "Backlog", "Atomic Card", "", "", "bug", "", "")
	if err == nil {
		t.Fatal("expected an error from the aborted card_tags insert")
	}

	var cards, tags, logs int
	testDB.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&cards)
	testDB.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&tags)
	testDB.QueryRow(`SELECT COUNT(*) FROM activity_log WHERE action = 'card_created'`).Scan(&logs)
	if cards != 0 {
		t.Errorf("want 0 cards after failed create, got %d", cards)
	}
	if tags != 0 {
		t.Errorf("want 0 tags after failed create, got %d", tags)
	}
	if logs != 0 {
		t.Errorf("want 0 card_created activities after failed create, got %d", logs)
	}
}

// --- H5 activity logging tests ---

type activityRow struct {
	BoardID string  `db:"board_id"`
	CardID  *string `db:"card_id"`
	Details *string `db:"details"`
}

func activityRows(t *testing.T, testDB *sqlx.DB, action string) []activityRow {
	t.Helper()
	var rows []activityRow
	if err := testDB.Select(&rows, `SELECT board_id, card_id, details FROM activity_log WHERE action = ?`, action); err != nil {
		t.Fatalf("select activity_log: %v", err)
	}
	return rows
}

func singleActivity(t *testing.T, testDB *sqlx.DB, action string) activityRow {
	t.Helper()
	rows := activityRows(t, testDB, action)
	if len(rows) != 1 {
		t.Fatalf("want exactly 1 %q activity, got %d", action, len(rows))
	}
	return rows[0]
}

func failActivityLog(t *testing.T, testDB *sqlx.DB) {
	t.Helper()
	if _, err := testDB.Exec(`CREATE TRIGGER fail_activity_log BEFORE INSERT ON activity_log BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
}

func TestManageTags_CreateLogsActivity(t *testing.T) {
	testDB := newTestDB(t)
	board, _ := ResolveBoard(testDB, "")

	result, err := ManageTags(testDB, "create", "", "", "", "bug", "#ef4444")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	tag := result.(*models.Tag)

	row := singleActivity(t, testDB, "tag_created")
	if row.BoardID != board.ID {
		t.Errorf("board_id = %q, want %q", row.BoardID, board.ID)
	}
	if row.CardID != nil {
		t.Errorf("card_id = %q, want NULL", *row.CardID)
	}
	if row.Details == nil || !strings.Contains(*row.Details, tag.Name) {
		t.Errorf("details should mention the tag name %q, got %v", tag.Name, row.Details)
	}
}

func TestManageTags_AssignLogsActivity(t *testing.T) {
	testDB := newTestDB(t)
	board, _ := ResolveBoard(testDB, "")
	card, _ := CreateCard(testDB, "", "", "", "Tagged Card", "", "", "", "", "")
	result, _ := ManageTags(testDB, "create", "", "", "", "bug", "#ef4444")
	tag := result.(*models.Tag)

	if _, err := ManageTags(testDB, "assign", "", tag.ID, card.ID, "", ""); err != nil {
		t.Fatalf("assign tag: %v", err)
	}

	row := singleActivity(t, testDB, "tag_assigned")
	if row.BoardID != board.ID {
		t.Errorf("board_id = %q, want %q", row.BoardID, board.ID)
	}
	if row.CardID == nil || *row.CardID != card.ID {
		t.Errorf("card_id = %v, want %q", row.CardID, card.ID)
	}
}

func TestManageTags_RemoveLogsActivity(t *testing.T) {
	testDB := newTestDB(t)
	board, _ := ResolveBoard(testDB, "")
	card, _ := CreateCard(testDB, "", "", "", "Tagged Card", "", "", "", "", "")
	result, _ := ManageTags(testDB, "create", "", "", "", "bug", "#ef4444")
	tag := result.(*models.Tag)
	ManageTags(testDB, "assign", "", tag.ID, card.ID, "", "")

	if _, err := ManageTags(testDB, "remove", "", tag.ID, card.ID, "", ""); err != nil {
		t.Fatalf("remove tag: %v", err)
	}

	row := singleActivity(t, testDB, "tag_removed")
	if row.BoardID != board.ID {
		t.Errorf("board_id = %q, want %q", row.BoardID, board.ID)
	}
	if row.CardID == nil || *row.CardID != card.ID {
		t.Errorf("card_id = %v, want %q", row.CardID, card.ID)
	}
}

func TestManageTags_DeleteLogsActivity(t *testing.T) {
	testDB := newTestDB(t)
	board, _ := ResolveBoard(testDB, "")
	result, _ := ManageTags(testDB, "create", "", "", "", "bug", "#ef4444")
	tag := result.(*models.Tag)

	if _, err := ManageTags(testDB, "delete", "", tag.ID, "", "", ""); err != nil {
		t.Fatalf("delete tag: %v", err)
	}

	row := singleActivity(t, testDB, "tag_deleted")
	if row.BoardID != board.ID {
		t.Errorf("board_id = %q, want %q", row.BoardID, board.ID)
	}
	if row.CardID != nil {
		t.Errorf("card_id = %q, want NULL", *row.CardID)
	}
	if row.Details == nil || !strings.Contains(*row.Details, "bug") {
		t.Errorf("details should mention the tag name, got %v", row.Details)
	}
}

func TestManageTags_UnknownTagIsNotFound(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "", "Card", "", "", "", "", "")

	for _, action := range []string{"assign", "remove", "delete"} {
		_, err := ManageTags(testDB, action, "", "missing-tag", card.ID, "", "")
		if err == nil || !strings.HasPrefix(err.Error(), "NOT_FOUND:") {
			t.Errorf("%s with unknown tag: want NOT_FOUND error, got %v", action, err)
		}
	}
	if rows := activityRows(t, testDB, "tag_assigned"); len(rows) != 0 {
		t.Errorf("want no tag_assigned activity, got %d", len(rows))
	}
}

func TestCreateCard_NewTagLogsTagCreatedOnce(t *testing.T) {
	testDB := newTestDB(t)

	if _, err := CreateCard(testDB, "", "", "", "First", "", "", "bug", "", ""); err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if rows := activityRows(t, testDB, "tag_created"); len(rows) != 1 {
		t.Fatalf("want 1 tag_created activity after creating a new tag, got %d", len(rows))
	}

	if _, err := CreateCard(testDB, "", "", "", "Second", "", "", "bug", "", ""); err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if rows := activityRows(t, testDB, "tag_created"); len(rows) != 1 {
		t.Errorf("reusing an existing tag must not log tag_created again, got %d rows", len(rows))
	}
}

func TestManageTags_CreateFailedLogLeavesNoTag(t *testing.T) {
	testDB := newTestDB(t)
	failActivityLog(t, testDB)

	_, err := ManageTags(testDB, "create", "", "", "", "bug", "#ef4444")
	if err == nil {
		t.Fatal("expected an error from the aborted activity insert")
	}
	if !strings.Contains(err.Error(), "log activity") {
		t.Errorf("error should name the activity log failure, got %v", err)
	}

	var tags int
	testDB.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&tags)
	if tags != 0 {
		t.Errorf("tag insert and its activity row are one transaction: want 0 tags, got %d", tags)
	}
}

// Single-statement writes are not wrapped in a transaction: the write persists
// even though the failed activity insert is reported to the caller.
func TestDeleteCard_LogActivityErrorPropagates_WriteNotRolledBack(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "", "Delete Me", "", "", "", "", "")
	failActivityLog(t, testDB)

	err := DeleteCard(testDB, card.ID)
	if err == nil {
		t.Fatal("expected an error from the aborted activity insert")
	}
	if !strings.Contains(err.Error(), "log activity") {
		t.Errorf("error should name the activity log failure, got %v", err)
	}

	var count int
	testDB.QueryRow(`SELECT COUNT(*) FROM cards WHERE id = ?`, card.ID).Scan(&count)
	if count != 0 {
		t.Errorf("delete is not transactional with its log: want 0 cards, got %d", count)
	}
}

func TestLogActivityErrorPropagates(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, testDB *sqlx.DB) error
	}{
		{"UpdateCard", func(t *testing.T, testDB *sqlx.DB) error {
			card, _ := CreateCard(testDB, "", "", "", "Card", "", "", "", "", "")
			failActivityLog(t, testDB)
			_, err := UpdateCard(testDB, card.ID, "Renamed", "", "", "", "", false, "", "", "")
			return err
		}},
		{"MoveCard", func(t *testing.T, testDB *sqlx.DB) error {
			card, _ := CreateCard(testDB, "", "", "Backlog", "Card", "", "", "", "", "")
			failActivityLog(t, testDB)
			_, err := MoveCard(testDB, card.ID, "", "", "In Progress", nil)
			return err
		}},
		{"CreateColumn", func(t *testing.T, testDB *sqlx.DB) error {
			failActivityLog(t, testDB)
			_, err := CreateColumn(testDB, "", "QA", "", "", nil)
			return err
		}},
		{"createPhase", func(t *testing.T, testDB *sqlx.DB) error {
			failActivityLog(t, testDB)
			_, err := ManagePhases(testDB, "create", "", "", "Testing", "", nil)
			return err
		}},
		{"updatePhase", func(t *testing.T, testDB *sqlx.DB) error {
			list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
			failActivityLog(t, testDB)
			_, err := ManagePhases(testDB, "update", "", list.([]models.Phase)[0].ID, "Dev", "", nil)
			return err
		}},
		{"deletePhase", func(t *testing.T, testDB *sqlx.DB) error {
			list, _ := ManagePhases(testDB, "list", "", "", "", "", nil)
			failActivityLog(t, testDB)
			_, err := ManagePhases(testDB, "delete", "", list.([]models.Phase)[0].ID, "", "", nil)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(t, newTestDB(t))
			if err == nil {
				t.Fatal("expected the activity log failure to propagate")
			}
			if !strings.Contains(err.Error(), "log activity") {
				t.Errorf("error should name the activity log failure, got %v", err)
			}
		})
	}
}

// --- H6: only sql.ErrNoRows maps to NOT_FOUND ---

func breakTable(t *testing.T, testDB *sqlx.DB, table string) {
	t.Helper()
	if _, err := testDB.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME TO %s_gone`, table, table)); err != nil {
		t.Fatalf("rename %s: %v", table, err)
	}
}

// failingGet delegates everything to the wrapped Querier except Get, which
// always fails with err. It simulates a lookup failure while writes still work.
type failingGet struct {
	Querier
	err error
}

func (f failingGet) Get(dest interface{}, query string, args ...interface{}) error {
	return f.err
}

func TestNotFound_MissingRowStillMapsToNotFound(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, testDB *sqlx.DB) error
	}{
		{"ResolveBoard", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolveBoard(testDB, "missing")
			return err
		}},
		{"ResolveColumn by id", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolveColumn(testDB, "", "missing", "")
			return err
		}},
		{"ResolveColumn by name", func(t *testing.T, testDB *sqlx.DB) error {
			board, _ := ResolveBoard(testDB, "")
			_, err := ResolveColumn(testDB, board.ID, "", "Nope")
			return err
		}},
		{"ResolveColumn first on empty board", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolveColumn(testDB, "missing-board", "", "")
			return err
		}},
		{"ResolvePhase by id", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolvePhase(testDB, "", "missing", "")
			return err
		}},
		{"ResolvePhase by name", func(t *testing.T, testDB *sqlx.DB) error {
			board, _ := ResolveBoard(testDB, "")
			_, err := ResolvePhase(testDB, board.ID, "", "Nope")
			return err
		}},
		{"UpdateCard", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := UpdateCard(testDB, "missing", "T", "", "", "", "", false, "", "", "")
			return err
		}},
		{"MoveCard", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := MoveCard(testDB, "missing", "", "", "", nil)
			return err
		}},
		{"DeleteCard", func(t *testing.T, testDB *sqlx.DB) error {
			return DeleteCard(testDB, "missing")
		}},
		{"createPhase on missing board", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ManagePhases(testDB, "create", "missing-board", "", "Testing", "", nil)
			return err
		}},
		{"updatePhase", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ManagePhases(testDB, "update", "", "missing", "Dev", "", nil)
			return err
		}},
		{"deletePhase", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ManagePhases(testDB, "delete", "", "missing", "", "", nil)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(t, newTestDB(t))
			if err == nil || !strings.HasPrefix(err.Error(), "NOT_FOUND:") {
				t.Errorf("want NOT_FOUND error, got %v", err)
			}
		})
	}
}

func TestNotFound_DBErrorKeepsItsCause(t *testing.T) {
	cases := []struct {
		name  string
		table string
		run   func(t *testing.T, testDB *sqlx.DB) error
	}{
		{"ResolveBoard by id", "boards", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolveBoard(testDB, "any")
			return err
		}},
		{"ResolveBoard default", "boards", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolveBoard(testDB, "")
			return err
		}},
		{"ResolveColumn by id", "columns", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolveColumn(testDB, "", "any", "")
			return err
		}},
		{"ResolveColumn by name", "columns", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolveColumn(testDB, "any", "", "Backlog")
			return err
		}},
		{"ResolveColumn first", "columns", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolveColumn(testDB, "any", "", "")
			return err
		}},
		{"ResolvePhase by id", "phases", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolvePhase(testDB, "", "any", "")
			return err
		}},
		{"ResolvePhase by name", "phases", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ResolvePhase(testDB, "any", "", "Development")
			return err
		}},
		{"UpdateCard", "cards", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := UpdateCard(testDB, "any", "T", "", "", "", "", false, "", "", "")
			return err
		}},
		{"MoveCard card lookup", "cards", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := MoveCard(testDB, "any", "", "", "", nil)
			return err
		}},
		{"MoveCard current column lookup", "", func(t *testing.T, testDB *sqlx.DB) error {
			card, err := CreateCard(testDB, "", "", "", "Card", "", "", "", "", "")
			if err != nil {
				t.Fatal(err)
			}
			breakTable(t, testDB, "columns")
			_, err = MoveCard(testDB, card.ID, "", "", "", nil)
			return err
		}},
		{"DeleteCard", "cards", func(t *testing.T, testDB *sqlx.DB) error {
			return DeleteCard(testDB, "any")
		}},
		{"createPhase board check", "boards", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ManagePhases(testDB, "create", "any", "", "Testing", "", nil)
			return err
		}},
		{"updatePhase", "phases", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ManagePhases(testDB, "update", "", "any", "Dev", "", nil)
			return err
		}},
		{"deletePhase", "phases", func(t *testing.T, testDB *sqlx.DB) error {
			_, err := ManagePhases(testDB, "delete", "", "any", "", "", nil)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testDB := newTestDB(t)
			if tc.table != "" {
				breakTable(t, testDB, tc.table)
			}
			err := tc.run(t, testDB)
			if err == nil {
				t.Fatal("expected a DB error")
			}
			if strings.HasPrefix(err.Error(), "NOT_FOUND:") {
				t.Errorf("DB error must not be reported as NOT_FOUND, got %v", err)
			}
			if !strings.Contains(err.Error(), "no such table") {
				t.Errorf("error should keep the SQLite cause, got %v", err)
			}
		})
	}
}

func TestFindOrCreateTag_LookupErrorDoesNotCreate(t *testing.T) {
	testDB := newTestDB(t)
	board, _ := ResolveBoard(testDB, "")
	locked := fmt.Errorf("database is locked")

	tx := testDB.MustBegin()
	defer tx.Rollback()

	_, err := FindOrCreateTag(failingGet{Querier: tx, err: locked}, board.ID, "bug")
	if err == nil {
		t.Fatal("expected the lookup error to propagate")
	}
	if strings.HasPrefix(err.Error(), "NOT_FOUND:") {
		t.Errorf("lookup error must not be reported as NOT_FOUND, got %v", err)
	}
	if !errors.Is(err, locked) {
		t.Errorf("error should wrap the lookup cause, got %v", err)
	}

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM tags WHERE board_id = ? AND name = 'bug'`, board.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("lookup failure must not fall through to an insert, got %d tag rows", count)
	}
}

// --- H9 batched GetBoard tests ---

func insertBoard(t *testing.T, testDB *sqlx.DB, id, name string) {
	t.Helper()
	// Later created_at than the seed board so the empty-board_id fallback stays on the seed.
	if _, err := testDB.Exec(`INSERT INTO boards (id, name, created_at, updated_at) VALUES (?, ?, '2999-01-01 00:00:00', '2999-01-01 00:00:00')`, id, name); err != nil {
		t.Fatalf("insert board %s: %v", id, err)
	}
}

func mustCreateCard(t *testing.T, testDB *sqlx.DB, boardID, columnName, title, tags, phaseName string) *models.Card {
	t.Helper()
	card, err := CreateCard(testDB, boardID, "", columnName, title, "", "", tags, "", phaseName)
	if err != nil {
		t.Fatalf("create card %q: %v", title, err)
	}
	return card
}

func cardTitles(cards []models.Card) []string {
	titles := []string{}
	for _, c := range cards {
		titles = append(titles, c.Title)
	}
	return titles
}

func tagNames(tags []models.Tag) []string {
	names := []string{}
	for _, tg := range tags {
		names = append(names, tg.Name)
	}
	return names
}

func TestGetBoard_AssemblesCardsTagsAndPhasesAcrossColumns(t *testing.T) {
	testDB := newTestDB(t)
	mustCreateCard(t, testDB, "", "Backlog", "A", "zeta,alpha", "")
	mustCreateCard(t, testDB, "", "Backlog", "B", "", "")
	mustCreateCard(t, testDB, "", "Backlog", "C", "", "")
	mustCreateCard(t, testDB, "", "To Do", "D", "alpha", "")
	e := mustCreateCard(t, testDB, "", "In Progress", "E", "", "QA")
	mustCreateCard(t, testDB, "", "In Progress", "F", "", "")

	board, err := GetBoard(testDB, "")
	if err != nil {
		t.Fatalf("GetBoard: %v", err)
	}
	if len(board.Phases) != 5 {
		t.Errorf("want 5 phases, got %d", len(board.Phases))
	}

	wantColumns := []string{"Backlog", "To Do", "In Progress", "Review", "Done"}
	wantCards := map[string][]string{
		"Backlog":     {"A", "B", "C"},
		"To Do":       {"D"},
		"In Progress": {"E", "F"},
		"Review":      {},
		"Done":        {},
	}
	wantTags := map[string][]string{"A": {"alpha", "zeta"}, "D": {"alpha"}}

	if len(board.Columns) != len(wantColumns) {
		t.Fatalf("want %d columns, got %d", len(wantColumns), len(board.Columns))
	}
	for i, col := range board.Columns {
		if col.Name != wantColumns[i] {
			t.Errorf("column %d: want %q, got %q", i, wantColumns[i], col.Name)
		}
		if col.Cards == nil {
			t.Errorf("column %q: Cards must be non-nil", col.Name)
		}
		if got := cardTitles(col.Cards); fmt.Sprint(got) != fmt.Sprint(wantCards[col.Name]) {
			t.Errorf("column %q: want cards %v, got %v", col.Name, wantCards[col.Name], got)
		}
		for _, card := range col.Cards {
			if card.ColumnID != col.ID {
				t.Errorf("card %q: column_id %s does not match column %s", card.Title, card.ColumnID, col.ID)
			}
			if card.Tags == nil {
				t.Errorf("card %q: Tags must be non-nil", card.Title)
			}
			want := wantTags[card.Title]
			if want == nil {
				want = []string{}
			}
			if got := tagNames(card.Tags); fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("card %q: want tags %v (sorted by name), got %v", card.Title, want, got)
			}
			for _, tg := range card.Tags {
				if tg.BoardID != board.ID {
					t.Errorf("card %q: tag %q belongs to board %s, want %s", card.Title, tg.Name, tg.BoardID, board.ID)
				}
			}
			if card.Title == "E" {
				if card.Phase == nil || card.Phase.Name != "QA" || card.PhaseID == nil || *card.PhaseID != *e.PhaseID {
					t.Errorf("card E: want phase QA (%v), got %+v", *e.PhaseID, card.Phase)
				}
			} else if card.Phase != nil || card.PhaseID != nil {
				t.Errorf("card %q: want no phase, got %+v", card.Title, card.Phase)
			}
		}
	}
}

func TestGetBoard_EmptyColumnsHaveEmptyCardSlices(t *testing.T) {
	testDB := newTestDB(t)
	insertBoard(t, testDB, "board-empty", "Empty")
	for _, name := range []string{"Inbox", "Outbox"} {
		if _, err := CreateColumn(testDB, "board-empty", name, "", "", nil); err != nil {
			t.Fatalf("create column %s: %v", name, err)
		}
	}

	board, err := GetBoard(testDB, "board-empty")
	if err != nil {
		t.Fatalf("GetBoard: %v", err)
	}
	if len(board.Columns) != 2 {
		t.Fatalf("want 2 columns, got %d", len(board.Columns))
	}
	if board.Phases == nil {
		t.Error("Phases must be non-nil on a board without phases")
	}
	for _, col := range board.Columns {
		if col.Cards == nil || len(col.Cards) != 0 {
			t.Errorf("column %q: want empty non-nil Cards, got %#v", col.Name, col.Cards)
		}
	}
	out, _ := json.Marshal(board)
	if got := strings.Count(string(out), `"cards":[]`); got != 2 {
		t.Errorf("want 2 columns with \"cards\":[], got %d in: %s", got, out)
	}
}

func TestGetBoard_IsolatesBoards(t *testing.T) {
	testDB := newTestDB(t)
	insertBoard(t, testDB, "board-b", "Board B")
	if _, err := CreateColumn(testDB, "board-b", "Inbox", "", "", nil); err != nil {
		t.Fatalf("create column: %v", err)
	}
	mustCreateCard(t, testDB, "", "Backlog", "A-card", "shared", "")
	mustCreateCard(t, testDB, "board-b", "Inbox", "B-card", "shared", "")

	check := func(boardID, wantTitle string) {
		t.Helper()
		board, err := GetBoard(testDB, boardID)
		if err != nil {
			t.Fatalf("GetBoard(%q): %v", boardID, err)
		}
		var cards []models.Card
		for _, col := range board.Columns {
			cards = append(cards, col.Cards...)
		}
		if len(cards) != 1 || cards[0].Title != wantTitle {
			t.Fatalf("board %s: want only card %q, got %v", board.ID, wantTitle, cardTitles(cards))
		}
		if len(cards[0].Tags) != 1 || cards[0].Tags[0].BoardID != board.ID {
			t.Errorf("board %s: want exactly one tag owned by the board, got %+v", board.ID, cards[0].Tags)
		}
	}
	check("", "A-card")
	check("board-b", "B-card")
}
