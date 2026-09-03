package hooks

import (
	"strings"
	"testing"
	"time"

	"github.com/juandagalo/cyber-mango-plugin-go/internal/services"
)

func TestStartReportRendersBoardSummaryAsPlainText(t *testing.T) {
	testDB := newTestDB(t)
	if _, err := services.CreateCard(testDB, "", "", "", "Fix login", "", "critical", "", "", ""); err != nil {
		t.Fatal(err)
	}

	out, err := StartReport(testDB)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Cyber Mango Board: Cyber Mango", "Total cards: 1", "Columns\n", "Priority Alerts", "CRITICAL: 1", "- [Backlog] Fix login"} {
		mustContain(t, out, want)
	}
	if strings.ContainsAny(out, "*#") {
		t.Fatal("start report must be plain text without markdown")
	}
}

func TestRecordSessionStartWithoutSessionIDWritesNothing(t *testing.T) {
	testDB := newTestDB(t)
	if err := RecordSessionStart(testDB, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := testDB.Get(&n, `SELECT COUNT(*) FROM _meta WHERE key LIKE 'stop_report:%'`); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected no watermark rows, found %d", n)
	}
}
