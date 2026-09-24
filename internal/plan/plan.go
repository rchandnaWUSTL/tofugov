package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	maxAttrsPerChange = 8
	maxValueLen       = 80
	maxDescribed      = 50
)

type ResourceChange struct {
	Address  string   `json:"address"`
	Type     string   `json:"type"`
	Action   string   `json:"action"`
	ForcedBy []string `json:"forced_by,omitempty"`
	Attrs    []string `json:"attrs,omitempty"`
}

type Facts struct {
	Creates       int              `json:"creates"`
	Updates       int              `json:"updates"`
	Deletes       int              `json:"deletes"`
	Replaces      int              `json:"replaces"`
	ReplaceAddrs  []string         `json:"replace_addrs,omitempty"`
	DeleteAddrs   []string         `json:"delete_addrs,omitempty"`
	ResourceTypes map[string]int   `json:"resource_types,omitempty"`
	Changes       []ResourceChange `json:"changes,omitempty"`
}

func (f Facts) NoOp() bool { return f.Creates+f.Updates+f.Deletes+f.Replaces == 0 }

func (f Facts) Counts() string {
	return fmt.Sprintf("%d to add, %d to change, %d to replace, %d to destroy", f.Creates, f.Updates, f.Replaces, f.Deletes)
}

type rawChange struct {
	Actions         []string                   `json:"actions"`
	Before          map[string]json.RawMessage `json:"before"`
	After           map[string]json.RawMessage `json:"after"`
	AfterUnknown    any                        `json:"after_unknown"`
	BeforeSensitive any                        `json:"before_sensitive"`
	AfterSensitive  any                        `json:"after_sensitive"`
	ReplacePaths    [][]any                    `json:"replace_paths"`
}

type rawPlan struct {
	ResourceChanges []struct {
		Address string    `json:"address"`
		Type    string    `json:"type"`
		Mode    string    `json:"mode"`
		Change  rawChange `json:"change"`
	} `json:"resource_changes"`
}

// Parse extracts deterministic facts from `tofu show -json <planfile>` output.
func Parse(b []byte) (Facts, error) {
	var raw rawPlan
	if err := json.Unmarshal(b, &raw); err != nil {
		return Facts{}, fmt.Errorf("parse plan json: %w", err)
	}
	f := Facts{ResourceTypes: map[string]int{}}
	for _, rc := range raw.ResourceChanges {
		if rc.Mode == "data" {
			continue
		}
		c := ResourceChange{Address: rc.Address, Type: rc.Type, Action: classify(rc.Change.Actions)}
		switch c.Action {
		case "create":
			f.Creates++
		case "update":
			f.Updates++
			c.Attrs = diffAttrs(rc.Change)
		case "delete":
			f.Deletes++
			f.DeleteAddrs = append(f.DeleteAddrs, rc.Address)
		case "replace":
			f.Replaces++
			f.ReplaceAddrs = append(f.ReplaceAddrs, rc.Address)
			c.Attrs = diffAttrs(rc.Change)
			c.ForcedBy = renderPaths(rc.Change.ReplacePaths)
		default:
			continue
		}
		f.ResourceTypes[rc.Type]++
		f.Changes = append(f.Changes, c)
	}
	return f, nil
}

func classify(actions []string) string {
	switch strings.Join(actions, ",") {
	case "create":
		return "create"
	case "update":
		return "update"
	case "delete":
		return "delete"
	case "delete,create", "create,delete":
		return "replace"
	}
	return ""
}

// diffAttrs lists changed top-level attributes, redacting sensitive values and
// skipping values that are only known after apply.
func diffAttrs(c rawChange) []string {
	seen := map[string]bool{}
	var keys []string
	for _, m := range []map[string]json.RawMessage{c.Before, c.After} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)

	var out []string
	for _, k := range keys {
		if hasTrue(sub(c.AfterUnknown, k)) {
			continue
		}
		before, after := compact(c.Before[k]), compact(c.After[k])
		if before == after {
			continue
		}
		if hasTrue(sub(c.BeforeSensitive, k)) || hasTrue(sub(c.AfterSensitive, k)) {
			out = append(out, k+": (sensitive value changed)")
		} else {
			out = append(out, fmt.Sprintf("%s: %s -> %s", k, clip(before), clip(after)))
		}
		if len(out) == maxAttrsPerChange {
			break
		}
	}
	return out
}

func sub(v any, key string) any {
	switch t := v.(type) {
	case bool:
		return t
	case map[string]any:
		return t[key]
	}
	return nil
}

func hasTrue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case map[string]any:
		for _, x := range t {
			if hasTrue(x) {
				return true
			}
		}
	case []any:
		for _, x := range t {
			if hasTrue(x) {
				return true
			}
		}
	}
	return false
}

func compact(m json.RawMessage) string {
	if len(m) == 0 {
		return "null"
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, m); err != nil {
		return string(m)
	}
	return buf.String()
}

func clip(s string) string {
	r := []rune(s)
	if len(r) <= maxValueLen {
		return s
	}
	return string(r[:maxValueLen-1]) + "…"
}

func renderPaths(paths [][]any) []string {
	var out []string
	for _, p := range paths {
		parts := make([]string, len(p))
		for i, el := range p {
			parts[i] = fmt.Sprint(el)
		}
		out = append(out, strings.Join(parts, "."))
	}
	return out
}

// Describe renders facts as the LLM-facing context. It never includes raw HCL
// or sensitive values.
func Describe(workspace, upgrade string, f Facts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Workspace: %s\n", workspace)
	if upgrade == "" {
		upgrade = "none (plan of the current configuration)"
	}
	fmt.Fprintf(&b, "Version upgrade included in this change: %s\n", upgrade)
	fmt.Fprintf(&b, "Plan totals: %s\n", f.Counts())
	if len(f.Changes) == 0 {
		b.WriteString("Resource changes: none\n")
		return b.String()
	}
	b.WriteString("Resource changes:\n")
	for i, c := range f.Changes {
		if i == maxDescribed {
			fmt.Fprintf(&b, "... and %d more\n", len(f.Changes)-i)
			break
		}
		fmt.Fprintf(&b, "- %s %s", c.Action, c.Address)
		if len(c.ForcedBy) > 0 {
			fmt.Fprintf(&b, " (replacement forced by: %s)", strings.Join(c.ForcedBy, ", "))
		}
		b.WriteString("\n")
		for _, a := range c.Attrs {
			fmt.Fprintf(&b, "    %s\n", a)
		}
	}
	return b.String()
}
