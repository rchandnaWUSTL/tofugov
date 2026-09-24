package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rchandnaWUSTL/tofugov/internal/plan"
)

const (
	ApplyNotApplied = "not_applied"
	ApplyApplied    = "applied"
	ApplyFailed     = "failed"
	ApplySkipped    = "skipped"

	OutcomeUnlabeled  = "unlabeled"
	OutcomeClean      = "clean"
	OutcomeRolledBack = "rolled_back"
	OutcomeIncident   = "incident"
	OutcomeOverridden = "overridden"
)

var Outcomes = []string{OutcomeClean, OutcomeRolledBack, OutcomeIncident, OutcomeOverridden}

func ValidOutcome(s string) bool {
	for _, o := range Outcomes {
		if s == o {
			return true
		}
	}
	return false
}

var ErrNotFound = errors.New("change not found")

type Change struct {
	ID          int64      `json:"id"`
	RunID       string     `json:"run_id"`
	Workspace   string     `json:"workspace"`
	Kind        string     `json:"kind"`
	Upgrade     string     `json:"upgrade,omitempty"`
	Facts       plan.Facts `json:"facts"`
	Summary     string     `json:"summary"`
	Explanation string     `json:"explanation"`
	PBad        *float64   `json:"p_bad"`
	Band        string     `json:"band"`
	Rationale   string     `json:"rationale,omitempty"`
	Model       string     `json:"model,omitempty"`
	Scored      bool       `json:"scored"`
	ScoreErr    string     `json:"score_error,omitempty"`
	ApplyStatus string     `json:"apply_status"`
	ApplyErr    string     `json:"apply_error,omitempty"`
	Outcome     string     `json:"outcome"`
	OutcomeNote string     `json:"outcome_note,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (c Change) Name() string { return filepath.Base(c.Workspace) }

const schema = `
CREATE TABLE IF NOT EXISTS changes (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id       TEXT NOT NULL,
  workspace    TEXT NOT NULL,
  kind         TEXT NOT NULL,
  upgrade      TEXT NOT NULL DEFAULT '',
  facts        TEXT NOT NULL,
  summary      TEXT NOT NULL DEFAULT '',
  explanation  TEXT NOT NULL DEFAULT '',
  p_bad        REAL,
  band         TEXT NOT NULL,
  rationale    TEXT NOT NULL DEFAULT '',
  model        TEXT NOT NULL DEFAULT '',
  scored       INTEGER NOT NULL,
  score_err    TEXT NOT NULL DEFAULT '',
  apply_status TEXT NOT NULL DEFAULT 'not_applied',
  apply_err    TEXT NOT NULL DEFAULT '',
  outcome      TEXT NOT NULL DEFAULT 'unlabeled',
  outcome_note TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS changes_workspace ON changes(workspace);
CREATE INDEX IF NOT EXISTS changes_run ON changes(run_id);
`

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Insert(c *Change) error {
	facts, err := json.Marshal(c.Facts)
	if err != nil {
		return err
	}
	if c.ApplyStatus == "" {
		c.ApplyStatus = ApplyNotApplied
	}
	if c.Outcome == "" {
		c.Outcome = OutcomeUnlabeled
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	res, err := s.db.Exec(`INSERT INTO changes
		(run_id, workspace, kind, upgrade, facts, summary, explanation, p_bad, band, rationale, model,
		 scored, score_err, apply_status, apply_err, outcome, outcome_note, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.RunID, c.Workspace, c.Kind, c.Upgrade, string(facts), c.Summary, c.Explanation, c.PBad, c.Band,
		c.Rationale, c.Model, c.Scored, c.ScoreErr, c.ApplyStatus, c.ApplyErr, c.Outcome, c.OutcomeNote,
		c.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	return err
}

func (s *Store) SetApply(id int64, status, applyErr string) error {
	return s.update(id, `UPDATE changes SET apply_status = ?, apply_err = ? WHERE id = ?`, status, applyErr, id)
}

func (s *Store) SetOutcome(id int64, outcome, note string) error {
	if outcome != OutcomeUnlabeled && !ValidOutcome(outcome) {
		return fmt.Errorf("invalid outcome %q (want one of %s)", outcome, strings.Join(Outcomes, ", "))
	}
	return s.update(id, `UPDATE changes SET outcome = ?, outcome_note = ? WHERE id = ?`, outcome, note, id)
}

func (s *Store) update(id int64, q string, args ...any) error {
	res, err := s.db.Exec(q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: #%d", ErrNotFound, id)
	}
	return nil
}

func (s *Store) Get(id int64) (*Change, error) {
	cs, err := s.query(`WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(cs) == 0 {
		return nil, fmt.Errorf("%w: #%d", ErrNotFound, id)
	}
	return &cs[0], nil
}

type Filter struct {
	Workspace string // matches a full path or a workspace directory name
	Band      string
	RunID     string
	Unscored  bool
	Limit     int
}

func (s *Store) List(f Filter) ([]Change, error) {
	var where []string
	var args []any
	if f.Workspace != "" {
		where = append(where, `(workspace = ? OR workspace LIKE ?)`)
		args = append(args, f.Workspace, "%/"+f.Workspace)
	}
	if f.Band != "" {
		where = append(where, `band = ?`)
		args = append(args, f.Band)
	}
	if f.RunID != "" {
		where = append(where, `run_id = ?`)
		args = append(args, f.RunID)
	}
	if f.Unscored {
		where = append(where, `scored = 0`)
	}
	q := ""
	if len(where) > 0 {
		q = "WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	return s.query(q, args...)
}

func (s *Store) query(tail string, args ...any) ([]Change, error) {
	rows, err := s.db.Query(`SELECT id, run_id, workspace, kind, upgrade, facts, summary, explanation, p_bad,
		band, rationale, model, scored, score_err, apply_status, apply_err, outcome, outcome_note, created_at
		FROM changes `+tail, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Change
	for rows.Next() {
		var c Change
		var facts, created string
		var pBad sql.NullFloat64
		if err := rows.Scan(&c.ID, &c.RunID, &c.Workspace, &c.Kind, &c.Upgrade, &facts, &c.Summary,
			&c.Explanation, &pBad, &c.Band, &c.Rationale, &c.Model, &c.Scored, &c.ScoreErr, &c.ApplyStatus,
			&c.ApplyErr, &c.Outcome, &c.OutcomeNote, &created); err != nil {
			return nil, err
		}
		if pBad.Valid {
			c.PBad = &pBad.Float64
		}
		if err := json.Unmarshal([]byte(facts), &c.Facts); err != nil {
			return nil, fmt.Errorf("change #%d: %w", c.ID, err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, c)
	}
	return out, rows.Err()
}
