package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rchandnaWUSTL/tofugov/internal/store"
	"github.com/rchandnaWUSTL/tofugov/internal/tofu"
)

type wsResult struct {
	Workspace string        `json:"workspace"`
	Error     string        `json:"error,omitempty"`
	Change    *store.Change `json:"change,omitempty"`
}

func (a *app) upgradeCmd() *cobra.Command {
	var (
		tofuVersion string
		providers   []string
		doApply     bool
		yes         bool
		onUnscored  string
	)
	cmd := &cobra.Command{
		Use:   "upgrade [flags] <workspace-dir>...",
		Short: "Bulk-upgrade OpenTofu and provider versions across workspaces, with shadow-mode risk scoring",
		Long: `Rewrites version constraints in each workspace, runs init -upgrade and plan,
then explains and risk-scores every plan. Scores are recorded but never gate
anything (shadow mode). Nothing is applied unless --apply is given.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if onUnscored != "skip" && onUnscored != "proceed" {
				return fmt.Errorf("--on-unscored must be skip or proceed")
			}
			target, desc, err := parseTarget(tofuVersion, providers)
			if err != nil {
				return err
			}
			dirs, err := workspaceDirs(args)
			if err != nil {
				return err
			}

			st, err := a.openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			p := a.newPipeline(st)
			ctx := cmd.Context()
			runID := time.Now().UTC().Format("20060102T150405Z")

			results := make([]wsResult, len(dirs))
			for i, dir := range dirs {
				results[i].Workspace = dir
				p.logf("[%d/%d] %s", i+1, len(dirs), filepath.Base(dir))
				changed, err := tofu.RewriteDir(dir, target)
				if err != nil {
					results[i].Error = err.Error()
					continue
				}
				if len(changed) == 0 {
					p.logf("  %s: no matching version constraints (already upgraded?); planning anyway", filepath.Base(dir))
				}
				ch, err := p.planAndScore(ctx, dir, "upgrade", runID, desc)
				if err != nil {
					results[i].Error = err.Error()
					p.logf("  %s: failed: %s", filepath.Base(dir), firstLine(err.Error()))
					continue
				}
				results[i].Change = ch
			}

			if !a.jsonOut {
				fmt.Fprintf(a.stdout, "\nRun %s: %s\n\n", runID, desc)
				printSummary(a.stdout, results)
			}
			if !doApply {
				if a.jsonOut {
					return a.printJSON(results)
				}
				fmt.Fprintln(a.stdout, "\nShadow mode: nothing was applied. Scores are recorded, not enforced.")
				fmt.Fprintln(a.stdout, "Review with `tofugov show <id>`; apply with --apply.")
				return failures(results)
			}

			if !yes {
				ok, err := confirm(fmt.Sprintf("\nApply %d workspace(s)? [y/N] ", countPlanned(results)))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(a.stdout, "Aborted; nothing applied.")
					return nil
				}
			}

			for i := range results {
				ch := results[i].Change
				if ch == nil {
					continue
				}
				status, applyErr := store.ApplyApplied, ""
				if !ch.Scored && onUnscored == "skip" {
					status, applyErr = store.ApplySkipped, "no risk score (verifier unavailable); rerun with --on-unscored=proceed to apply anyway"
					p.logf("%s: skipped, %s", ch.Name(), applyErr)
				} else {
					p.logf("%s: tofu apply", ch.Name())
					if err := p.tofu.Apply(ctx, ch.Workspace); err != nil {
						status, applyErr = store.ApplyFailed, err.Error()
						p.logf("%s: apply failed: %s", ch.Name(), firstLine(applyErr))
					}
				}
				if err := st.SetApply(ch.ID, status, applyErr); err != nil {
					return err
				}
				ch.ApplyStatus, ch.ApplyErr = status, applyErr
				if status == store.ApplyApplied {
					if err := st.SetOutcome(ch.ID, store.OutcomeClean, "auto-labeled: apply succeeded"); err != nil {
						return err
					}
					ch.Outcome = store.OutcomeClean
				}
			}

			if a.jsonOut {
				return a.printJSON(results)
			}
			fmt.Fprintln(a.stdout, "\nResults")
			printApplyResults(a.stdout, results)
			fmt.Fprintln(a.stdout, "\nApplied changes are auto-labeled clean. Relabel with `tofugov outcome <id> --result incident|rolled_back|overridden`.")
			return failures(results)
		},
	}
	f := cmd.Flags()
	f.StringVar(&tofuVersion, "tofu-version", "", `constraint to write to required_version, e.g. ">= 1.8.0"`)
	f.StringArrayVar(&providers, "provider", nil, `provider constraint as namespace/name=constraint, e.g. hashicorp/random=3.7.2 (repeatable)`)
	f.BoolVar(&doApply, "apply", false, "apply the saved plans after showing the summary")
	f.BoolVarP(&yes, "yes", "y", false, "skip the apply confirmation prompt")
	f.StringVar(&onUnscored, "on-unscored", "skip", "what --apply does with plans the verifier could not score: skip or proceed")
	return cmd
}

func parseTarget(tofuVersion string, providers []string) (tofu.Target, string, error) {
	t := tofu.Target{TofuVersion: tofuVersion, Providers: map[string]string{}}
	var desc []string
	if tofuVersion != "" {
		desc = append(desc, "opentofu "+tofuVersion)
	}
	for _, p := range providers {
		src, c, ok := strings.Cut(p, "=")
		src, c = strings.TrimSpace(src), strings.TrimSpace(c)
		if !ok || c == "" || strings.Count(tofu.NormalizeSource(src), "/") != 1 {
			return t, "", fmt.Errorf("invalid --provider %q (want namespace/name=constraint)", p)
		}
		t.Providers[tofu.NormalizeSource(src)] = c
	}
	var srcs []string
	for s := range t.Providers {
		srcs = append(srcs, s)
	}
	sort.Strings(srcs)
	for _, s := range srcs {
		desc = append(desc, s+" -> "+t.Providers[s])
	}
	if t.Empty() {
		return t, "", fmt.Errorf("nothing to upgrade: pass --tofu-version and/or --provider")
	}
	return t, strings.Join(desc, ", "), nil
}

func workspaceDirs(args []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, a := range args {
		abs, err := filepath.Abs(a)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", a)
		}
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	return out, nil
}

func confirm(prompt string) (bool, error) {
	if info, err := os.Stdin.Stat(); err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false, fmt.Errorf("refusing to apply without confirmation in a non-interactive shell; pass --yes")
	}
	fmt.Fprint(os.Stderr, prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes", nil
}

func countPlanned(rs []wsResult) int {
	n := 0
	for _, r := range rs {
		if r.Change != nil {
			n++
		}
	}
	return n
}

func failures(rs []wsResult) error {
	n := 0
	for _, r := range rs {
		if r.Error != "" || (r.Change != nil && r.Change.ApplyStatus == store.ApplyFailed) {
			n++
		}
	}
	if n > 0 {
		return fmt.Errorf("%d workspace(s) failed", n)
	}
	return nil
}
