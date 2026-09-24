package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rchandnaWUSTL/tofugov/internal/report"
	"github.com/rchandnaWUSTL/tofugov/internal/store"
)

func (a *app) scoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "score <workspace-dir>",
		Short: "Plan one workspace, then explain and shadow-score the plan (never applies)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dirs, err := workspaceDirs(args)
			if err != nil {
				return err
			}
			st, err := a.openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			runID := time.Now().UTC().Format("20060102T150405Z")
			ch, err := a.newPipeline(st).planAndScore(cmd.Context(), dirs[0], "score", runID, "")
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(ch)
			}
			printChange(a.stdout, ch)
			return nil
		},
	}
}

func (a *app) showCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <change-id>",
		Short: "Show a recorded change: plan facts, explanation, score, and outcome",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			st, err := a.openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			ch, err := st.Get(id)
			if err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(ch)
			}
			printChange(a.stdout, ch)
			return nil
		},
	}
}

func (a *app) outcomeCmd() *cobra.Command {
	var result, note string
	cmd := &cobra.Command{
		Use:   "outcome <change-id> --result <outcome>",
		Short: "Label what actually happened after a change (feeds calibration)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if !store.ValidOutcome(result) {
				return fmt.Errorf("--result must be one of %s", strings.Join(store.Outcomes, ", "))
			}
			st, err := a.openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			if err := st.SetOutcome(id, result, note); err != nil {
				return err
			}
			if a.jsonOut {
				return a.printJSON(map[string]any{"id": id, "outcome": result, "note": note})
			}
			fmt.Fprintf(a.stdout, "Change #%d labeled %s\n", id, result)
			return nil
		},
	}
	cmd.Flags().StringVar(&result, "result", "", "one of "+strings.Join(store.Outcomes, ", "))
	cmd.Flags().StringVar(&note, "note", "", "free-text context, e.g. an incident link")
	cmd.MarkFlagRequired("result")
	return cmd
}

func (a *app) historyCmd() *cobra.Command {
	var f store.Filter
	cmd := &cobra.Command{
		Use:   "history",
		Short: "List recorded changes, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := a.openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			cs, err := st.List(f)
			if err != nil {
				return err
			}
			if a.jsonOut {
				if cs == nil {
					cs = []store.Change{}
				}
				return a.printJSON(cs)
			}
			printHistory(a.stdout, cs)
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.Workspace, "workspace", "", "workspace directory name or path")
	fl.StringVar(&f.Band, "band", "", "low, medium, high, or unscored")
	fl.StringVar(&f.RunID, "run", "", "run ID")
	fl.BoolVar(&f.Unscored, "unscored", false, "only changes the verifier could not score")
	fl.IntVar(&f.Limit, "limit", 50, "maximum rows (0 for all)")
	return cmd
}

func (a *app) reportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Calibration report: predicted risk vs. labeled outcomes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := a.openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			cs, err := st.List(store.Filter{})
			if err != nil {
				return err
			}
			r := report.Build(cs)
			if a.jsonOut {
				return a.printJSON(r)
			}
			printReport(a.stdout, r)
			return nil
		},
	}
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimPrefix(s, "#"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid change id %q", s)
	}
	return id, nil
}
