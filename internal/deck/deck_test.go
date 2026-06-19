package deck

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "decklist.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLoadValid(t *testing.T) {
	path := write(t, `{
		"plane": "prod",
		"spells": [{"name": "ready", "phase": "untap", "cast": ["docker", "info"]}]
	}`)

	d, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if d.Plane != "prod" || len(d.Spells) != 1 {
		t.Fatalf("unexpected decklist: %+v", d)
	}
}

func TestLoadRejectsNoSpells(t *testing.T) {
	path := write(t, `{"plane": "prod", "spells": []}`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for empty spells")
	}
}

func TestLoadRejectsMissingCast(t *testing.T) {
	path := write(t, `{"plane":"prod","spells":[{"name":"x","phase":"untap","cast":[]}]}`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing cast")
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	path := write(t, `{"plane":"prod","mana":7,"spells":[{"name":"x","phase":"untap","cast":["true"]}]}`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for unknown field")
	}
}
