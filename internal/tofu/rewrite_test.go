package tofu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const versionsTF = `terraform {
  required_version = ">= 1.6.0"
  required_providers {
    random = {
      source  = "hashicorp/random"
      version = "~> 3.5"
    }
    local = {
      source  = "registry.opentofu.org/hashicorp/local"
      version = ">= 2.4, < 3"
    }
    null = { source = "hashicorp/null", version = "= 3.2.1" }
    other = {
      source  = "acme/other"
      version = "1.0.0"
    }
  }
}

module "vpc" {
  source  = "acme/vpc/aws"
  version = "1.0.0"
}
`

func TestRewrite(t *testing.T) {
	got := Rewrite(versionsTF, Target{
		TofuVersion: ">= 1.8.0",
		Providers: map[string]string{
			"hashicorp/random": "3.7.2",
			"hashicorp/local":  "~> 2.5",
			"hashicorp/null":   "3.2.4",
		},
	})
	for _, want := range []string{
		`required_version = ">= 1.8.0"`,
		`version = "3.7.2"`,
		`version = "~> 2.5"`,
		`null = { source = "hashicorp/null", version = "3.2.4" }`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Count(got, `version = "1.0.0"`) != 2 {
		t.Errorf("unrelated provider and module versions must be untouched:\n%s", got)
	}
}

func TestRewriteNoTargetIsIdentity(t *testing.T) {
	if Rewrite(versionsTF, Target{}) != versionsTF {
		t.Fatal("empty target changed the file")
	}
}

func TestRewriteDirKeepsFirstBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "versions.tf")
	if err := os.WriteFile(path, []byte(versionsTF), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"3.6.0", "3.7.2"} {
		if _, err := RewriteDir(dir, Target{Providers: map[string]string{"hashicorp/random": v}}); err != nil {
			t.Fatal(err)
		}
	}
	backup, _ := os.ReadFile(filepath.Join(dir, ".tofugov", "backup", "versions.tf"))
	if string(backup) != versionsTF {
		t.Fatalf("backup should hold the original file, got:\n%s", backup)
	}
	changed, err := RewriteDir(dir, Target{Providers: map[string]string{"hashicorp/random": "3.7.2"}})
	if err != nil || len(changed) != 0 {
		t.Fatalf("rewrite to same version should be a no-op: %v %v", changed, err)
	}
}
