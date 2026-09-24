package store

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/rchandnaWUSTL/tofugov/internal/plan"
)

func TestRoundTripAndFilters(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "sub", "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	p := 0.7
	scored := &Change{RunID: "r1", Workspace: "/x/ws-06-db", Kind: "upgrade", PBad: &p, Band: "high", Scored: true,
		Facts: plan.Facts{Replaces: 1, ReplaceAddrs: []string{"random_password.db"}}}
	unscored := &Change{RunID: "r1", Workspace: "/x/ws-01-net", Kind: "upgrade", Band: "unscored", ScoreErr: "no key"}
	for _, c := range []*Change{scored, unscored} {
		if err := s.Insert(c); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.Get(scored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PBad == nil || *got.PBad != 0.7 || got.Facts.ReplaceAddrs[0] != "random_password.db" || got.Outcome != OutcomeUnlabeled {
		t.Fatalf("round trip mismatch: %+v", got)
	}

	for _, tc := range []struct {
		f    Filter
		want int
	}{
		{Filter{}, 2},
		{Filter{Workspace: "ws-06-db"}, 1},
		{Filter{Band: "high"}, 1},
		{Filter{Unscored: true}, 1},
		{Filter{RunID: "nope"}, 0},
		{Filter{Limit: 1}, 1},
	} {
		cs, err := s.List(tc.f)
		if err != nil {
			t.Fatal(err)
		}
		if len(cs) != tc.want {
			t.Errorf("List(%+v) = %d rows, want %d", tc.f, len(cs), tc.want)
		}
	}

	if err := s.SetOutcome(scored.ID, OutcomeIncident, "pager"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetOutcome(scored.ID, "bogus", ""); err == nil {
		t.Fatal("expected invalid outcome error")
	}
	if err := s.SetOutcome(999, OutcomeClean, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
