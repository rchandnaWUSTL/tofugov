package report

import (
	"math"
	"testing"

	"github.com/rchandnaWUSTL/tofugov/internal/store"
)

func c(p float64, outcome string) store.Change {
	return store.Change{Scored: true, PBad: &p, Outcome: outcome}
}

func TestBuild(t *testing.T) {
	r := Build([]store.Change{
		c(0.05, store.OutcomeClean),
		c(0.05, store.OutcomeClean),
		c(0.95, store.OutcomeIncident),
		c(1.0, store.OutcomeOverridden),
		c(0.5, store.OutcomeUnlabeled),
		{Scored: false, Outcome: store.OutcomeClean},
	})

	if r.Labeled != 4 || r.Bad != 2 || r.UnlabeledScored != 1 || r.Unscored != 1 {
		t.Fatalf("counts: %+v", r)
	}
	if len(r.Buckets) != 2 {
		t.Fatalf("buckets: %+v", r.Buckets)
	}
	top := r.Buckets[1]
	if top.Lo != 0.9 || top.N != 2 || top.ObservedRate() != 1 || math.Abs(top.MeanPred-0.975) > 1e-9 {
		t.Errorf("p=1.0 must land in the top bucket: %+v", top)
	}
	wantBrier := (0.0025 + 0.0025 + 0.0025 + 0) / 4
	if math.Abs(r.Brier-wantBrier) > 1e-9 {
		t.Errorf("Brier = %v, want %v", r.Brier, wantBrier)
	}
	if r.BaselineBrier != 0.25 {
		t.Errorf("BaselineBrier = %v", r.BaselineBrier)
	}
}
