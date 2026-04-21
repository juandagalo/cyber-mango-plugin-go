package services

import (
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
	col, err := CreateColumn(testDB, "", "QA", "#ff0000", nil)
	if err != nil {
		t.Fatalf("CreateColumn: %v", err)
	}
	if col.Name != "QA" {
		t.Errorf("want column name 'QA', got %q", col.Name)
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
