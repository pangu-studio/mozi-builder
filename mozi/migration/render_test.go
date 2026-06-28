package migration

import (
	"strings"
	"testing"
)

func TestRenderSafe(t *testing.T) {
	files, err := RenderSafe(Plan{Steps: []Step{{Kind: "add_column", Risk: RiskSafe, SQL: `ALTER TABLE "decks" ADD COLUMN "color" TEXT;`, Reversible: true}}}, "20260628", "add color")
	if err != nil || files.Up == "" || files.Down == "" {
		t.Fatalf("files=%#v err=%v", files, err)
	}
	if !strings.Contains(files.Down, `DROP COLUMN "color"`) {
		t.Fatalf("down=%s", files.Down)
	}
}

func TestRenderSafeRejectsConditional(t *testing.T) {
	_, err := RenderSafe(Plan{Steps: []Step{{Kind: "rename_column", Risk: RiskConditional, RequiresConfirmation: true}}}, "1", "rename")
	if err == nil {
		t.Fatal("expected review gate")
	}
}
