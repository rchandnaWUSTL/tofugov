package report

import "github.com/rchandnaWUSTL/tofugov/internal/store"

type Bucket struct {
	Lo, Hi   float64
	N, Bad   int
	MeanPred float64
}

func (b Bucket) ObservedRate() float64 { return float64(b.Bad) / float64(b.N) }

type Report struct {
	Buckets         []Bucket
	Labeled         int
	Bad             int
	UnlabeledScored int
	Unscored        int
	Brier           float64
	// BaselineBrier is the Brier score of always predicting the observed base
	// rate; a verifier with real signal should beat it.
	BaselineBrier float64
}

func IsBad(outcome string) bool {
	return outcome == store.OutcomeRolledBack || outcome == store.OutcomeIncident || outcome == store.OutcomeOverridden
}

func Build(changes []store.Change) Report {
	var r Report
	var buckets [10]Bucket
	for i := range buckets {
		buckets[i].Lo, buckets[i].Hi = float64(i)/10, float64(i+1)/10
	}

	sumSq := 0.0
	for _, c := range changes {
		switch {
		case !c.Scored || c.PBad == nil:
			r.Unscored++
			continue
		case c.Outcome == store.OutcomeUnlabeled:
			r.UnlabeledScored++
			continue
		}
		p, y := *c.PBad, 0.0
		if IsBad(c.Outcome) {
			y = 1
			r.Bad++
		}
		i := min(int(p*10), 9)
		buckets[i].N++
		buckets[i].MeanPred += p
		buckets[i].Bad += int(y)
		sumSq += (p - y) * (p - y)
		r.Labeled++
	}

	for _, b := range buckets {
		if b.N > 0 {
			b.MeanPred /= float64(b.N)
			r.Buckets = append(r.Buckets, b)
		}
	}
	if r.Labeled > 0 {
		r.Brier = sumSq / float64(r.Labeled)
		base := float64(r.Bad) / float64(r.Labeled)
		r.BaselineBrier = base * (1 - base)
	}
	return r
}
