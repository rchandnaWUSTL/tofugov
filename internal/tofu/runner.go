package tofu

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PlanFile is relative to the workspace directory.
const PlanFile = ".tofugov/plan.tfplan"

type Runner struct {
	Bin string
}

func (r Runner) run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.Bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TF_IN_AUTOMATION=1", "TF_INPUT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("%s %s: %w\n%s", r.Bin, args[0], err, lastLines(msg, 15))
	}
	return stdout.Bytes(), nil
}

func (r Runner) Init(ctx context.Context, dir string) error {
	_, err := r.run(ctx, dir, "init", "-upgrade", "-input=false", "-no-color")
	return err
}

func (r Runner) Plan(ctx context.Context, dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(PlanFile)), 0o755); err != nil {
		return err
	}
	_, err := r.run(ctx, dir, "plan", "-input=false", "-no-color", "-out="+PlanFile)
	return err
}

func (r Runner) ShowJSON(ctx context.Context, dir string) ([]byte, error) {
	return r.run(ctx, dir, "show", "-json", "-no-color", PlanFile)
}

func (r Runner) Apply(ctx context.Context, dir string) error {
	_, err := r.run(ctx, dir, "apply", "-input=false", "-no-color", PlanFile)
	return err
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
