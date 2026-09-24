package tofu

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Target describes the version constraints an upgrade writes. Values are
// written verbatim, so "3.7.2" pins and "~> 3.7" constrains.
type Target struct {
	TofuVersion string
	Providers   map[string]string // "namespace/name" -> constraint
}

func (t Target) Empty() bool { return t.TofuVersion == "" && len(t.Providers) == 0 }

var (
	requiredVersionRE = regexp.MustCompile(`(\brequired_version\s*=\s*)"[^"]*"`)
	braceObjectRE     = regexp.MustCompile(`\{[^{}]*\}`)
	sourceRE          = regexp.MustCompile(`\bsource\s*=\s*"([^"]*)"`)
	versionAttrRE     = regexp.MustCompile(`(\bversion\s*=\s*)"[^"]*"`)
)

// Rewrite updates required_version and the version of each matching
// required_providers entry. Entries without a version attribute are left alone.
func Rewrite(src string, t Target) string {
	out := src
	if t.TofuVersion != "" {
		out = replaceValue(requiredVersionRE, out, t.TofuVersion)
	}
	if len(t.Providers) == 0 {
		return out
	}
	return braceObjectRE.ReplaceAllStringFunc(out, func(obj string) string {
		m := sourceRE.FindStringSubmatch(obj)
		if m == nil {
			return obj
		}
		c, ok := t.Providers[normalizeSource(m[1])]
		if !ok {
			return obj
		}
		return replaceValue(versionAttrRE, obj, c)
	})
}

func replaceValue(re *regexp.Regexp, s, value string) string {
	return re.ReplaceAllStringFunc(s, func(m string) string {
		return re.FindStringSubmatch(m)[1] + `"` + value + `"`
	})
}

// normalizeSource drops a registry hostname so "registry.opentofu.org/hashicorp/random"
// and "hashicorp/random" match the same flag.
func normalizeSource(s string) string {
	s = strings.ToLower(s)
	if parts := strings.Split(s, "/"); len(parts) == 3 && strings.Contains(parts[0], ".") {
		return parts[1] + "/" + parts[2]
	}
	return s
}

func NormalizeSource(s string) string { return normalizeSource(s) }

// RewriteDir applies Rewrite to every *.tf file in dir, backing up each
// original to .tofugov/backup the first time it is changed.
func RewriteDir(dir string, t Target) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return changed, err
		}
		updated := Rewrite(string(b), t)
		if updated == string(b) {
			continue
		}
		info, err := os.Stat(f)
		if err != nil {
			return changed, err
		}
		backup := filepath.Join(dir, ".tofugov", "backup", filepath.Base(f))
		if _, err := os.Stat(backup); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
				return changed, err
			}
			if err := os.WriteFile(backup, b, info.Mode().Perm()); err != nil {
				return changed, fmt.Errorf("backup %s: %w", f, err)
			}
		}
		if err := os.WriteFile(f, []byte(updated), info.Mode().Perm()); err != nil {
			return changed, err
		}
		changed = append(changed, filepath.Base(f))
	}
	return changed, nil
}
