package explain

import (
	"context"
	"errors"
	"strings"

	"github.com/rchandnaWUSTL/tofugov/internal/llm"
	"github.com/rchandnaWUSTL/tofugov/internal/plan"
)

type Result struct {
	Summary string
	Text    string
}

type Explainer struct {
	LLM *llm.Client
}

const system = `You explain OpenTofu infrastructure plans to engineers and to non-experts who do not read HCL.
Write plain English. Do not use HCL or plan-diff syntax.
Name every resource that will be replaced or destroyed by its full address, and say what that means in practice (for example data loss, credential rotation, downtime, or side effects from provisioners).
If the plan changes nothing, say so plainly.
Respond with JSON only: {"summary": "<one line, under 100 characters>", "explanation": "<2 to 6 sentences>"}`

func (e Explainer) Explain(ctx context.Context, workspace, upgrade string, f plan.Facts) (Result, error) {
	var out struct {
		Summary     string `json:"summary"`
		Explanation string `json:"explanation"`
	}
	if err := e.LLM.JSON(ctx, system, plan.Describe(workspace, upgrade, f), &out); err != nil {
		return Result{}, err
	}
	text := strings.TrimSpace(out.Explanation)
	if text == "" {
		return Result{}, errors.New("model returned an empty explanation")
	}
	summary := oneLine(out.Summary)
	if summary == "" {
		summary = oneLine(text)
	}
	return Result{Summary: summary, Text: Ground(text, f)}, nil
}

// Fallback is a deterministic explanation used when the LLM is unavailable.
func Fallback(f plan.Facts) Result {
	s := "Plan: " + f.Counts() + "."
	if f.NoOp() {
		s = "No infrastructure changes."
	}
	return Result{Summary: s, Text: Ground(s, f)}
}

// Ground appends any replaced or destroyed address the text failed to mention,
// so destructive actions are always called out even if the model omits one.
func Ground(text string, f plan.Facts) string {
	var missing []string
	for _, a := range f.ReplaceAddrs {
		if !strings.Contains(text, a) {
			missing = append(missing, "- replace "+a)
		}
	}
	for _, a := range f.DeleteAddrs {
		if !strings.Contains(text, a) {
			missing = append(missing, "- destroy "+a)
		}
	}
	if len(missing) == 0 {
		return text
	}
	return text + "\n\nDestructive actions:\n" + strings.Join(missing, "\n")
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}
