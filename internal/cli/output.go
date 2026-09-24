package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rchandnaWUSTL/tofugov/internal/report"
	"github.com/rchandnaWUSTL/tofugov/internal/store"
)

func table(w io.Writer) *tabwriter.Writer { return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0) }

func pBad(c *store.Change) string {
	if c.PBad == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *c.PBad)
}

func band(c *store.Change) string { return strings.ToUpper(c.Band) }

func planCounts(c *store.Change) string {
	f := c.Facts
	return fmt.Sprintf("+%d ~%d ±%d -%d", f.Creates, f.Updates, f.Replaces, f.Deletes)
}

// summaryRank orders failures and unscored plans first, then riskiest first,
// so the rows that need a human are at the top.
func summaryRank(r wsResult) float64 {
	switch {
	case r.Change == nil:
		return 3
	case r.Change.PBad == nil:
		return 2
	}
	return *r.Change.PBad
}

func printSummary(w io.Writer, rs []wsResult) {
	rs = append([]wsResult(nil), rs...)
	sort.SliceStable(rs, func(i, j int) bool { return summaryRank(rs[i]) > summaryRank(rs[j]) })
	t := table(w)
	fmt.Fprintln(t, "ID\tWORKSPACE\tPLAN\tRISK\tP_BAD\tSUMMARY")
	for _, r := range rs {
		if r.Change == nil {
			fmt.Fprintf(t, "-\t%s\t-\tFAILED\t-\t%s\n", filepath.Base(r.Workspace), clip(firstLine(r.Error), 70))
			continue
		}
		c := r.Change
		fmt.Fprintf(t, "%d\t%s\t%s\t%s\t%s\t%s\n", c.ID, c.Name(), planCounts(c), band(c), pBad(c), clip(c.Summary, 70))
	}
	t.Flush()
}

func printApplyResults(w io.Writer, rs []wsResult) {
	t := table(w)
	fmt.Fprintln(t, "ID\tWORKSPACE\tRISK\tRESULT\tDETAIL")
	for _, r := range rs {
		if r.Change == nil {
			fmt.Fprintf(t, "-\t%s\t-\tfailed (plan)\t%s\n", filepath.Base(r.Workspace), clip(firstLine(r.Error), 70))
			continue
		}
		c := r.Change
		fmt.Fprintf(t, "%d\t%s\t%s\t%s\t%s\n", c.ID, c.Name(), band(c), c.ApplyStatus, clip(firstLine(c.ApplyErr), 70))
	}
	t.Flush()
}

func printHistory(w io.Writer, cs []store.Change) {
	if len(cs) == 0 {
		fmt.Fprintln(w, "No changes recorded.")
		return
	}
	t := table(w)
	fmt.Fprintln(t, "ID\tWHEN\tWORKSPACE\tKIND\tPLAN\tRISK\tP_BAD\tAPPLY\tOUTCOME\tSUMMARY")
	for i := range cs {
		c := &cs[i]
		fmt.Fprintf(t, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", c.ID, c.CreatedAt.Local().Format("Jan 02 15:04"),
			c.Name(), c.Kind, planCounts(c), band(c), pBad(c), c.ApplyStatus, c.Outcome, clip(c.Summary, 50))
	}
	t.Flush()
}

func printChange(w io.Writer, c *store.Change) {
	fmt.Fprintf(w, "Change #%d  %s  (%s, run %s)\n", c.ID, c.Name(), c.Kind, c.RunID)
	if c.Upgrade != "" {
		fmt.Fprintf(w, "Upgrade:    %s\n", c.Upgrade)
	}
	fmt.Fprintf(w, "Plan:       %s\n", c.Facts.Counts())
	if c.Scored {
		fmt.Fprintf(w, "Risk:       %s  p_bad=%s  (%s, shadow mode: not enforced)\n", band(c), pBad(c), c.Model)
		if c.Rationale != "" {
			fmt.Fprintf(w, "Rationale:  %s\n", c.Rationale)
		}
	} else {
		fmt.Fprintf(w, "Risk:       unscored (%s)\n", firstLine(c.ScoreErr))
	}
	fmt.Fprintf(w, "Apply:      %s\n", c.ApplyStatus)
	fmt.Fprintf(w, "Outcome:    %s", c.Outcome)
	if c.OutcomeNote != "" {
		fmt.Fprintf(w, " (%s)", c.OutcomeNote)
	}
	fmt.Fprintf(w, "\n\nExplanation:\n%s\n", indent(c.Explanation))
	if len(c.Facts.Changes) > 0 {
		fmt.Fprintln(w, "\nResource changes:")
		for _, rc := range c.Facts.Changes {
			fmt.Fprintf(w, "  %-8s %s", rc.Action, rc.Address)
			if len(rc.ForcedBy) > 0 {
				fmt.Fprintf(w, "  (forced by: %s)", strings.Join(rc.ForcedBy, ", "))
			}
			fmt.Fprintln(w)
			for _, at := range rc.Attrs {
				fmt.Fprintf(w, "             %s\n", at)
			}
		}
	}
}

func printReport(w io.Writer, r report.Report) {
	fmt.Fprintf(w, "Labeled changes: %d (bad: %d)   Scored but unlabeled: %d   Unscored: %d\n\n",
		r.Labeled, r.Bad, r.UnlabeledScored, r.Unscored)
	if r.Labeled == 0 {
		fmt.Fprintln(w, "No labeled outcomes yet. Apply changes or label them with `tofugov outcome`.")
		return
	}
	t := table(w)
	fmt.Fprintln(t, "PREDICTED P_BAD\tN\tMEAN PREDICTED\tOBSERVED BAD RATE")
	for _, b := range r.Buckets {
		fmt.Fprintf(t, "%.1f–%.1f\t%d\t%.2f\t%.2f\n", b.Lo, b.Hi, b.N, b.MeanPred, b.ObservedRate())
	}
	t.Flush()
	fmt.Fprintf(w, "\nBrier score: %.3f  (baseline, always predicting the %.0f%% base rate: %.3f; lower is better)\n",
		r.Brier, 100*float64(r.Bad)/float64(r.Labeled), r.BaselineBrier)
	if r.Labeled < 20 {
		fmt.Fprintln(w, "Fewer than 20 labeled changes: too few to judge calibration.")
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func indent(s string) string {
	return "  " + strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n  ")
}
