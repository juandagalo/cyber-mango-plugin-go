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
	card, err := CreateCard(testDB, "", "", "Backlog", "Test Card", "A description", "high", "")
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
	_, err := CreateCard(testDB, "", "", "", "Bad Card", "", "urgent", "")
	if err == nil {
		t.Error("expected error for invalid priority")
	}
}

func TestUpdateCard(t *testing.T) {
	testDB := newTestDB(t)
	card, _ := CreateCard(testDB, "", "", "", "Original", "", "medium", "")
	updated, err := UpdateCard(testDB, card.ID, "Updated Title", "", "high")
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
	card, _ := CreateCard(testDB, "", "", "Backlog", "Move Me", "", "", "")

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
	card, _ := CreateCard(testDB, "", "", "", "Delete Me", "", "", "")
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
	card, _ := CreateCard(testDB, "", "", "", "Tagged Card", "", "", "")

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
	card, err := CreateCard(testDB, "", "", "", "Tagged Task", "", "medium", "my-project,bug")
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

	card, err := CreateCard(testDB, "", "", "", "Second Task", "", "medium", "my-project")
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
	card, err := CreateCard(testDB, "", "", "", "Trimmed Task", "", "medium", " feature , , docs ")
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
