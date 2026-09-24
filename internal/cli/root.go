package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rchandnaWUSTL/tofugov/internal/explain"
	"github.com/rchandnaWUSTL/tofugov/internal/llm"
	"github.com/rchandnaWUSTL/tofugov/internal/store"
	"github.com/rchandnaWUSTL/tofugov/internal/tofu"
	"github.com/rchandnaWUSTL/tofugov/internal/verify"
)

const DefaultModel = "qwen/qwen3-235b-a22b-2507"

type app struct {
	dbPath     string
	model      string
	tofuBin    string
	jsonOut    bool
	thresholds verify.Thresholds
	stdout     io.Writer
	stderr     io.Writer
}

func NewRoot() *cobra.Command {
	loadDotEnv(".env")

	a := &app{stdout: os.Stdout, stderr: os.Stderr, thresholds: verify.DefaultThresholds}
	model := os.Getenv("TOFUGOV_MODEL")
	if model == "" {
		model = DefaultModel
	}

	root := &cobra.Command{
		Use:           "tofugov",
		Short:         "Shadow-mode risk scoring and bulk version upgrades for OpenTofu workspaces",
		SilenceUsage:  true,
		SilenceErrors: false,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.thresholds.Validate()
		},
	}
	pf := root.PersistentFlags()
	dbPath := os.Getenv("TOFUGOV_DB")
	if dbPath == "" {
		dbPath = ".tofugov/tofugov.db"
	}
	pf.StringVar(&a.dbPath, "db", dbPath, "path to the change history database (env TOFUGOV_DB)")
	pf.StringVar(&a.model, "model", model, "OpenRouter model for explanations and scoring (env TOFUGOV_MODEL)")
	pf.StringVar(&a.tofuBin, "tofu", "tofu", "OpenTofu binary")
	pf.BoolVar(&a.jsonOut, "json", false, "print JSON instead of tables")
	pf.Float64Var(&a.thresholds.Medium, "medium-threshold", verify.DefaultThresholds.Medium, "p_bad at or above which a change is medium risk")
	pf.Float64Var(&a.thresholds.High, "high-threshold", verify.DefaultThresholds.High, "p_bad at or above which a change is high risk")

	root.AddCommand(a.upgradeCmd(), a.scoreCmd(), a.showCmd(), a.outcomeCmd(), a.historyCmd(), a.reportCmd())
	return root
}

func (a *app) openStore() (*store.Store, error) { return store.Open(a.dbPath) }

func (a *app) newPipeline(st *store.Store) *pipeline {
	client := llm.New(os.Getenv("OPENROUTER_API_KEY"), a.model)
	return &pipeline{
		tofu:      tofu.Runner{Bin: a.tofuBin},
		explainer: explain.Explainer{LLM: client},
		verifier:  verify.OpenRouter{LLM: client, Thresholds: a.thresholds},
		store:     st,
		log:       a.stderr,
	}
}

func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// loadDotEnv sets KEY=VALUE pairs from path without overriding variables
// already present in the environment.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if _, set := os.LookupEnv(k); !set && v != "" {
			os.Setenv(k, v)
		}
	}
}
