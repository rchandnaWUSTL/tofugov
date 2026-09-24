package explain

import (
	"strings"
	"testing"

	"github.com/rchandnaWUSTL/tofugov/internal/plan"
)

func TestGroundAppendsOnlyMissingAddresses(t *testing.T) {
	f := plan.Facts{
		ReplaceAddrs: []string{"random_password.db_master", "null_resource.migrate"},
		DeleteAddrs:  []string{"local_file.export[0]"},
	}
	got := Ground("This rotates random_password.db_master.", f)
	if strings.Contains(got, "- replace random_password.db_master") {
		t.Errorf("mentioned address should not be repeated:\n%s", got)
	}
	for _, want := range []string{"- replace null_resource.migrate", "- destroy local_file.export[0]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestGroundLeavesCompleteTextAlone(t *testing.T) {
	f := plan.Facts{DeleteAddrs: []string{"a.b"}}
	if got := Ground("Destroys a.b.", f); got != "Destroys a.b." {
		t.Errorf("got %q", got)
	}
}

func TestFallbackNoOp(t *testing.T) {
	if got := Fallback(plan.Facts{}); got.Text != "No infrastructure changes." {
		t.Errorf("got %q", got.Text)
	}
}
