package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/rchandnaWUSTL/tofugov/internal/explain"
	"github.com/rchandnaWUSTL/tofugov/internal/plan"
	"github.com/rchandnaWUSTL/tofugov/internal/store"
	"github.com/rchandnaWUSTL/tofugov/internal/tofu"
	"github.com/rchandnaWUSTL/tofugov/internal/verify"
)

type pipeline struct {
	tofu      tofu.Runner
	explainer explain.Explainer
	verifier  verify.Verifier
	store     *store.Store
	log       io.Writer
}

// planAndScore plans a workspace, explains and scores the plan, and records
// the result. Explanation and scoring failures are recorded, not returned:
// shadow mode must never block on the verifier.
func (p *pipeline) planAndScore(ctx context.Context, dir, kind, runID, upgrade string) (*store.Change, error) {
	name := filepath.Base(dir)
	p.logf("  %s: tofu init", name)
	if err := p.tofu.Init(ctx, dir); err != nil {
		return nil, err
	}
	p.logf("  %s: tofu plan", name)
	if err := p.tofu.Plan(ctx, dir); err != nil {
		return nil, err
	}
	raw, err := p.tofu.ShowJSON(ctx, dir)
	if err != nil {
		return nil, err
	}
	facts, err := plan.Parse(raw)
	if err != nil {
		return nil, err
	}

	p.logf("  %s: explaining and scoring (%s)", name, facts.Counts())
	ex, err := p.explainer.Explain(ctx, name, upgrade, facts)
	if err != nil {
		p.logf("  %s: explanation unavailable, using plan facts: %s", name, clip(firstLine(err.Error()), 160))
		ex = explain.Fallback(facts)
		ex.Text = "[LLM explanation unavailable]\n" + ex.Text
	}

	ch := &store.Change{
		RunID: runID, Workspace: dir, Kind: kind, Upgrade: upgrade,
		Facts: facts, Summary: ex.Summary, Explanation: ex.Text,
	}
	s, err := p.verifier.Score(ctx, verify.Input{Workspace: name, Upgrade: upgrade, Facts: facts, Explanation: ex.Text})
	if err != nil {
		ch.Band = string(verify.Unscored)
		ch.ScoreErr = err.Error()
		p.logf("  %s: not scored (%s)", name, clip(firstLine(err.Error()), 160))
	} else {
		pb := s.PBad
		ch.Scored, ch.PBad, ch.Band, ch.Rationale, ch.Model = true, &pb, string(s.Band), s.Rationale, s.Model
	}

	if err := p.store.Insert(ch); err != nil {
		return nil, fmt.Errorf("record change: %w", err)
	}
	return ch, nil
}

func (p *pipeline) logf(format string, args ...any) {
	fmt.Fprintf(p.log, format+"\n", args...)
}
