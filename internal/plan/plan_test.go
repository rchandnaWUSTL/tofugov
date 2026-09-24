package plan

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	b, err := os.ReadFile("testdata/show.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}

	if f.Creates != 1 || f.Updates != 1 || f.Replaces != 2 || f.Deletes != 1 {
		t.Fatalf("counts = %s", f.Counts())
	}
	if want := []string{"random_password.db_master", "local_sensitive_file.pgpass"}; !reflect.DeepEqual(f.ReplaceAddrs, want) {
		t.Errorf("ReplaceAddrs = %v, want %v", f.ReplaceAddrs, want)
	}
	if want := []string{"local_file.legacy_export[0]"}; !reflect.DeepEqual(f.DeleteAddrs, want) {
		t.Errorf("DeleteAddrs = %v, want %v", f.DeleteAddrs, want)
	}
	if len(f.Changes) != 5 {
		t.Fatalf("expected no-op and data sources to be skipped, got %d changes", len(f.Changes))
	}

	pw := f.Changes[1]
	if !reflect.DeepEqual(pw.ForcedBy, []string{"keepers"}) {
		t.Errorf("ForcedBy = %v", pw.ForcedBy)
	}
	if want := []string{`keepers: {"rotation":"2025-q4"} -> {"rotation":"2026-q3"}`}; !reflect.DeepEqual(pw.Attrs, want) {
		t.Errorf("Attrs = %v, want %v (unknown-after values must be skipped)", pw.Attrs, want)
	}

	cfg := f.Changes[0]
	if want := []string{"api_token: (sensitive value changed)", "input: 3 -> 4"}; !reflect.DeepEqual(cfg.Attrs, want) {
		t.Errorf("Attrs = %v, want %v", cfg.Attrs, want)
	}
}

func TestDescribeNeverLeaksSensitiveValues(t *testing.T) {
	b, _ := os.ReadFile("testdata/show.json")
	f, _ := Parse(b)
	d := Describe("ws", "hashicorp/random -> 3.7.2", f)
	for _, secret := range []string{"hunter2", "s3cret-old", "s3cret-new"} {
		if strings.Contains(d, secret) {
			t.Errorf("Describe leaked %q:\n%s", secret, d)
		}
	}
	if !strings.Contains(d, "replace random_password.db_master (replacement forced by: keepers)") {
		t.Errorf("missing replace line:\n%s", d)
	}
}
