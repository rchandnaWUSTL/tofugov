package verify

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/rchandnaWUSTL/tofugov/internal/llm"
	"github.com/rchandnaWUSTL/tofugov/internal/plan"
)

type Band string

const (
	Low      Band = "low"
	Medium   Band = "medium"
	High     Band = "high"
	Unscored Band = "unscored"
)

type Thresholds struct {
	Medium, High float64
}

var DefaultThresholds = Thresholds{Medium: 0.15, High: 0.5}

func (t Thresholds) Band(p float64) Band {
	switch {
	case p >= t.High:
		return High
	case p >= t.Medium:
		return Medium
	}
	return Low
}

func (t Thresholds) Validate() error {
	if !(0 < t.Medium && t.Medium < t.High && t.High <= 1) {
		return fmt.Errorf("thresholds must satisfy 0 < medium (%v) < high (%v) <= 1", t.Medium, t.High)
	}
	return nil
}

type Input struct {
	Workspace   string
	Upgrade     string
	Facts       plan.Facts
	Explanation string
}

type Score struct {
	PBad      float64
	Band      Band
	Rationale string
	Model     string
}

// Verifier returns a probability that a change leads to a bad outcome. It is
// the seam where a calibrated verifier such as Jev plugs in.
type Verifier interface {
	Score(ctx context.Context, in Input) (Score, error)
}

type OpenRouter struct {
	LLM        *llm.Client
	Thresholds Thresholds
}

const system = `You are a change-risk verifier for OpenTofu infrastructure plans.
Estimate the probability that applying this exact plan leads to a bad outcome: it gets rolled back, it causes an incident, or a careful human reviewer would reject or override it.
Weigh destroyed or replaced stateful resources (databases, data stores, credentials), side-effecting provisioners, blast radius, and reversibility.
A plan with no resource changes is near-zero risk. A routine in-place tweak is low risk.
Give your best estimate; do not inflate or hedge.
Respond with JSON only: {"p_bad": <number from 0 to 1>, "rationale": "<one or two sentences>"}`

func (v OpenRouter) Score(ctx context.Context, in Input) (Score, error) {
	user := plan.Describe(in.Workspace, in.Upgrade, in.Facts) + "\nExplanation given to the requester:\n" + in.Explanation
	var raw rawScore
	if err := v.LLM.JSON(ctx, system, user, &raw); err != nil {
		return Score{}, err
	}
	s, err := parse(raw, v.Thresholds)
	if err != nil {
		return Score{}, err
	}
	s.Model = v.LLM.Model
	return s, nil
}

type rawScore struct {
	PBad      *float64 `json:"p_bad"`
	Rationale string   `json:"rationale"`
}

func parse(r rawScore, t Thresholds) (Score, error) {
	if r.PBad == nil {
		return Score{}, errors.New("verifier response missing p_bad")
	}
	p := *r.PBad
	if math.IsNaN(p) || math.IsInf(p, 0) {
		return Score{}, fmt.Errorf("verifier returned invalid p_bad %v", p)
	}
	p = math.Min(1, math.Max(0, p))
	return Score{PBad: p, Band: t.Band(p), Rationale: strings.TrimSpace(r.Rationale)}, nil
}
