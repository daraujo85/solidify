// Package storage — persistência SQLite (modernc.org/sqlite, pure-Go).
//
// CGO_ENABLED=0 OK. DB único em <baseDir>/solidify.db. Schema versionado
// em schema_version.
package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SchemaVersion atual.
const SchemaVersion = 1

// DBPrefix "file:" + options comuns para storage local.
const DBPrefix = "file:"

// Store é um wrapper do *sql.DB com helpers tipados por domínio.
type Store struct {
	db *sql.DB
}

// Open abre (ou cria) o banco. path deve apontar para um arquivo .db.
// WAL mode habilitado para reduzir locking conflicts.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("storage: path vazio")
	}
	dsn := DBPrefix + path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage: ping: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage: migrate: %w", err)
	}
	return s, nil
}

// Close fecha o banco.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB devolve o *sql.DB subjacente. Útil para queries customizadas.
func (s *Store) DB() *sql.DB { return s.db }

// migrate aplica o schema se a versão é 0.
func (s *Store) migrate() error {
	_, err := s.db.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO schema_version (version) VALUES (?)`, SchemaVersion)
	return err
}

// schemaSQL define as tabelas. Idempotente (CREATE IF NOT EXISTS).
const schemaSQL = `
CREATE TABLE IF NOT EXISTS schema_version (
  version INTEGER PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS runs (
  id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  finished_at INTEGER,
  base_ref TEXT,
  head_ref TEXT,
  status TEXT NOT NULL,
  meta TEXT
);

CREATE TABLE IF NOT EXISTS components (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  name TEXT NOT NULL,
  framework TEXT,
  language TEXT,
  paths TEXT,
  prr_score REAL,
  classification TEXT,
  metadata TEXT,
  FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS analyzer_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  component_id INTEGER,
  analyzer TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at INTEGER,
  finished_at INTEGER,
  error TEXT,
  data TEXT,
  FOREIGN KEY (run_id) REFERENCES runs(id),
  FOREIGN KEY (component_id) REFERENCES components(id)
);

CREATE TABLE IF NOT EXISTS reviews (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  model TEXT NOT NULL,
  prompt TEXT,
  response TEXT,
  started_at INTEGER,
  finished_at INTEGER,
  tokens_in INTEGER,
  tokens_out INTEGER,
  cost REAL,
  verdict TEXT,
  FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS models (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT,
  declared TEXT,
  effective TEXT,
  notes TEXT,
  FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE TABLE IF NOT EXISTS findings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  review_id INTEGER,
  severity TEXT NOT NULL,
  category TEXT NOT NULL,
  message TEXT NOT NULL,
  location TEXT,
  evidence TEXT,
  source TEXT,
  FOREIGN KEY (run_id) REFERENCES runs(id),
  FOREIGN KEY (review_id) REFERENCES reviews(id)
);

CREATE TABLE IF NOT EXISTS artifacts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL,
  name TEXT NOT NULL,
  size INTEGER NOT NULL,
  sha256 TEXT NOT NULL,
  written_at INTEGER NOT NULL,
  manifest_path TEXT,
  FOREIGN KEY (run_id) REFERENCES runs(id)
);

CREATE INDEX IF NOT EXISTS idx_components_run ON components(run_id);
CREATE INDEX IF NOT EXISTS idx_analyzer_results_run ON analyzer_results(run_id);
CREATE INDEX IF NOT EXISTS idx_reviews_run ON reviews(run_id);
CREATE INDEX IF NOT EXISTS idx_models_run ON models(run_id);
CREATE INDEX IF NOT EXISTS idx_findings_run ON findings(run_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_run ON artifacts(run_id);
`

// Run representa um run gravado.
type Run struct {
	ID         string
	CreatedAt  time.Time
	FinishedAt *time.Time
	BaseRef    string
	HeadRef    string
	Status     string
	Meta       map[string]string
}

// InsertRun grava um run. Meta é serializado como JSON.
func (s *Store) InsertRun(r Run) error {
	if r.ID == "" {
		return fmt.Errorf("storage: run.id vazio")
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	if r.Status == "" {
		r.Status = "running"
	}
	metaJSON, err := json.Marshal(r.Meta)
	if err != nil {
		return fmt.Errorf("storage: marshal meta: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO runs (id, created_at, finished_at, base_ref, head_ref, status, meta)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.CreatedAt.Unix(), nullTime(r.FinishedAt), r.BaseRef, r.HeadRef, r.Status, string(metaJSON),
	)
	if err != nil {
		return fmt.Errorf("storage: insert run: %w", err)
	}
	return nil
}

// FinishRun marca finished_at e status.
func (s *Store) FinishRun(id, status string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE runs SET finished_at = ?, status = ? WHERE id = ?`, now.Unix(), status, id)
	return err
}

// GetRun carrega um run por ID.
func (s *Store) GetRun(id string) (Run, error) {
	row := s.db.QueryRow(`SELECT id, created_at, finished_at, base_ref, head_ref, status, meta FROM runs WHERE id = ?`, id)
	return scanRun(row)
}

// ListRuns devolve todos os runs, ordenados por created_at desc.
func (s *Store) ListRuns() ([]Run, error) {
	rows, err := s.db.Query(`SELECT id, created_at, finished_at, base_ref, head_ref, status, meta FROM runs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner) (Run, error) {
	var r Run
	var created int64
	var finished sql.NullInt64
	var meta sql.NullString
	if err := row.Scan(&r.ID, &created, &finished, &r.BaseRef, &r.HeadRef, &r.Status, &meta); err != nil {
		if err == sql.ErrNoRows {
			return Run{}, ErrNotFound
		}
		return Run{}, err
	}
	r.CreatedAt = time.Unix(created, 0).UTC()
	if finished.Valid {
		t := time.Unix(finished.Int64, 0).UTC()
		r.FinishedAt = &t
	}
	if meta.Valid && meta.String != "" {
		_ = json.Unmarshal([]byte(meta.String), &r.Meta)
	}
	return r, nil
}

// Component é um component detectado no run.
type Component struct {
	ID             int64
	RunID          string
	Name           string
	Framework      string
	Language       string
	Paths          []string
	PrrScore       float64
	Classification string
	Metadata       map[string]string
}

// InsertComponent grava um component.
func (s *Store) InsertComponent(c Component) (int64, error) {
	if c.RunID == "" {
		return 0, fmt.Errorf("storage: run_id vazio")
	}
	if c.Name == "" {
		return 0, fmt.Errorf("storage: name vazio")
	}
	pathsJSON, _ := json.Marshal(c.Paths)
	metaJSON, _ := json.Marshal(c.Metadata)
	res, err := s.db.Exec(
		`INSERT INTO components (run_id, name, framework, language, paths, prr_score, classification, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.RunID, c.Name, nullStr(c.Framework), nullStr(c.Language), string(pathsJSON),
		c.PrrScore, nullStr(c.Classification), string(metaJSON),
	)
	if err != nil {
		return 0, fmt.Errorf("storage: insert component: %w", err)
	}
	return res.LastInsertId()
}

// ListComponentsByRun devolve components de um run.
func (s *Store) ListComponentsByRun(runID string) ([]Component, error) {
	rows, err := s.db.Query(`SELECT id, run_id, name, framework, language, paths, prr_score, classification, metadata FROM components WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Component
	for rows.Next() {
		c, err := scanComponent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanComponent(row rowScanner) (Component, error) {
	var c Component
	var fw, lang, paths, cls, meta sql.NullString
	if err := row.Scan(&c.ID, &c.RunID, &c.Name, &fw, &lang, &paths, &c.PrrScore, &cls, &meta); err != nil {
		return Component{}, err
	}
	if fw.Valid {
		c.Framework = fw.String
	}
	if lang.Valid {
		c.Language = lang.String
	}
	if cls.Valid {
		c.Classification = cls.String
	}
	if paths.Valid && paths.String != "" {
		_ = json.Unmarshal([]byte(paths.String), &c.Paths)
	}
	if meta.Valid && meta.String != "" {
		_ = json.Unmarshal([]byte(meta.String), &c.Metadata)
	}
	return c, nil
}

// AnalyzerResult registra uma execução de analyzer.
type AnalyzerResult struct {
	ID          int64
	RunID       string
	ComponentID *int64
	Analyzer    string
	Status      string
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Error       string
	Data        map[string]any
}

// InsertAnalyzerResult grava um analyzer_result.
func (s *Store) InsertAnalyzerResult(a AnalyzerResult) (int64, error) {
	if a.RunID == "" {
		return 0, fmt.Errorf("storage: run_id vazio")
	}
	if a.Analyzer == "" {
		return 0, fmt.Errorf("storage: analyzer vazio")
	}
	if a.Status == "" {
		a.Status = "pending"
	}
	dataJSON, _ := json.Marshal(a.Data)
	res, err := s.db.Exec(
		`INSERT INTO analyzer_results (run_id, component_id, analyzer, status, started_at, finished_at, error, data)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.RunID, nullInt64(a.ComponentID), a.Analyzer, a.Status,
		nullTime(a.StartedAt), nullTime(a.FinishedAt), nullStr(a.Error), string(dataJSON),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListAnalyzerResultsByRun devolve resultados por run.
func (s *Store) ListAnalyzerResultsByRun(runID string) ([]AnalyzerResult, error) {
	rows, err := s.db.Query(`SELECT id, run_id, component_id, analyzer, status, started_at, finished_at, error, data FROM analyzer_results WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AnalyzerResult
	for rows.Next() {
		a, err := scanAnalyzerResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanAnalyzerResult(row rowScanner) (AnalyzerResult, error) {
	var a AnalyzerResult
	var compID sql.NullInt64
	var started, finished sql.NullInt64
	var errStr, data sql.NullString
	if err := row.Scan(&a.ID, &a.RunID, &compID, &a.Analyzer, &a.Status, &started, &finished, &errStr, &data); err != nil {
		return AnalyzerResult{}, err
	}
	if compID.Valid {
		v := compID.Int64
		a.ComponentID = &v
	}
	if started.Valid {
		t := time.Unix(started.Int64, 0).UTC()
		a.StartedAt = &t
	}
	if finished.Valid {
		t := time.Unix(finished.Int64, 0).UTC()
		a.FinishedAt = &t
	}
	if errStr.Valid {
		a.Error = errStr.String
	}
	if data.Valid && data.String != "" {
		_ = json.Unmarshal([]byte(data.String), &a.Data)
	}
	return a, nil
}

// Review é uma chamada a um modelo (geralmente LLM).
type Review struct {
	ID         int64
	RunID      string
	Model      string
	Prompt     string
	Response   string
	StartedAt  *time.Time
	FinishedAt *time.Time
	TokensIn   int
	TokensOut  int
	Cost       float64
	Verdict    string
}

// InsertReview grava uma review.
func (s *Store) InsertReview(r Review) (int64, error) {
	if r.RunID == "" {
		return 0, fmt.Errorf("storage: run_id vazio")
	}
	if r.Model == "" {
		return 0, fmt.Errorf("storage: model vazio")
	}
	res, err := s.db.Exec(
		`INSERT INTO reviews (run_id, model, prompt, response, started_at, finished_at, tokens_in, tokens_out, cost, verdict)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RunID, r.Model, r.Prompt, r.Response, nullTime(r.StartedAt), nullTime(r.FinishedAt),
		r.TokensIn, r.TokensOut, r.Cost, nullStr(r.Verdict),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetReview carrega review por ID.
func (s *Store) GetReview(id int64) (Review, error) {
	row := s.db.QueryRow(`SELECT id, run_id, model, prompt, response, started_at, finished_at, tokens_in, tokens_out, cost, verdict FROM reviews WHERE id = ?`, id)
	return scanReview(row)
}

// ListReviewsByRun devolve reviews por run.
func (s *Store) ListReviewsByRun(runID string) ([]Review, error) {
	rows, err := s.db.Query(`SELECT id, run_id, model, prompt, response, started_at, finished_at, tokens_in, tokens_out, cost, verdict FROM reviews WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		r, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanReview(row rowScanner) (Review, error) {
	var r Review
	var started, finished sql.NullInt64
	var verdict sql.NullString
	if err := row.Scan(&r.ID, &r.RunID, &r.Model, &r.Prompt, &r.Response, &started, &finished, &r.TokensIn, &r.TokensOut, &r.Cost, &verdict); err != nil {
		if err == sql.ErrNoRows {
			return Review{}, ErrNotFound
		}
		return Review{}, err
	}
	if started.Valid {
		t := time.Unix(started.Int64, 0).UTC()
		r.StartedAt = &t
	}
	if finished.Valid {
		t := time.Unix(finished.Int64, 0).UTC()
		r.FinishedAt = &t
	}
	if verdict.Valid {
		r.Verdict = verdict.String
	}
	return r, nil
}

// Model é a descrição de um modelo usado num run.
type Model struct {
	ID        int64
	RunID     string
	Name      string
	Role      string
	Declared  string
	Effective string
	Notes     string
}

// InsertModel grava um model.
func (s *Store) InsertModel(m Model) (int64, error) {
	if m.RunID == "" {
		return 0, fmt.Errorf("storage: run_id vazio")
	}
	if m.Name == "" {
		return 0, fmt.Errorf("storage: name vazio")
	}
	res, err := s.db.Exec(
		`INSERT INTO models (run_id, name, role, declared, effective, notes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.RunID, m.Name, nullStr(m.Role), nullStr(m.Declared), nullStr(m.Effective), nullStr(m.Notes),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListModelsByRun devolve models por run.
func (s *Store) ListModelsByRun(runID string) ([]Model, error) {
	rows, err := s.db.Query(`SELECT id, run_id, name, role, declared, effective, notes FROM models WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Model
	for rows.Next() {
		m, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanModel(row rowScanner) (Model, error) {
	var m Model
	var role, declared, effective, notes sql.NullString
	if err := row.Scan(&m.ID, &m.RunID, &m.Name, &role, &declared, &effective, &notes); err != nil {
		return Model{}, err
	}
	if role.Valid {
		m.Role = role.String
	}
	if declared.Valid {
		m.Declared = declared.String
	}
	if effective.Valid {
		m.Effective = effective.String
	}
	if notes.Valid {
		m.Notes = notes.String
	}
	return m, nil
}

// Finding é uma constatação de um analyzer ou review.
type Finding struct {
	ID       int64
	RunID    string
	ReviewID *int64
	Severity string
	Category string
	Message  string
	Location string
	Evidence string
	Source   string
}

// InsertFinding grava um finding.
func (s *Store) InsertFinding(f Finding) (int64, error) {
	if f.RunID == "" {
		return 0, fmt.Errorf("storage: run_id vazio")
	}
	if f.Severity == "" {
		return 0, fmt.Errorf("storage: severity vazio")
	}
	if f.Category == "" {
		return 0, fmt.Errorf("storage: category vazio")
	}
	if f.Message == "" {
		return 0, fmt.Errorf("storage: message vazio")
	}
	res, err := s.db.Exec(
		`INSERT INTO findings (run_id, review_id, severity, category, message, location, evidence, source)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.RunID, nullInt64(f.ReviewID), f.Severity, f.Category, f.Message,
		nullStr(f.Location), nullStr(f.Evidence), nullStr(f.Source),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListFindingsByRun devolve findings por run.
func (s *Store) ListFindingsByRun(runID string) ([]Finding, error) {
	rows, err := s.db.Query(`SELECT id, run_id, review_id, severity, category, message, location, evidence, source FROM findings WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FindingsBySeverity agrupa findings de um run por severity.
func (s *Store) FindingsBySeverity(runID string) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT severity, COUNT(*) FROM findings WHERE run_id = ? GROUP BY severity`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var sev string
		var n int
		if err := rows.Scan(&sev, &n); err != nil {
			return nil, err
		}
		out[sev] = n
	}
	return out, rows.Err()
}

func scanFinding(row rowScanner) (Finding, error) {
	var f Finding
	var reviewID sql.NullInt64
	var loc, ev, src sql.NullString
	if err := row.Scan(&f.ID, &f.RunID, &reviewID, &f.Severity, &f.Category, &f.Message, &loc, &ev, &src); err != nil {
		return Finding{}, err
	}
	if reviewID.Valid {
		v := reviewID.Int64
		f.ReviewID = &v
	}
	if loc.Valid {
		f.Location = loc.String
	}
	if ev.Valid {
		f.Evidence = ev.String
	}
	if src.Valid {
		f.Source = src.String
	}
	return f, nil
}

// Artifact registra um artefato do run.
type Artifact struct {
	ID           int64
	RunID        string
	Name         string
	Size         int64
	SHA256       string
	WrittenAt    time.Time
	ManifestPath string
}

// InsertArtifact grava um artifact.
func (s *Store) InsertArtifact(a Artifact) (int64, error) {
	if a.RunID == "" {
		return 0, fmt.Errorf("storage: run_id vazio")
	}
	if a.Name == "" {
		return 0, fmt.Errorf("storage: name vazio")
	}
	if a.SHA256 == "" {
		return 0, fmt.Errorf("storage: sha256 vazio")
	}
	if a.WrittenAt.IsZero() {
		a.WrittenAt = time.Now().UTC()
	}
	res, err := s.db.Exec(
		`INSERT INTO artifacts (run_id, name, size, sha256, written_at, manifest_path)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		a.RunID, a.Name, a.Size, a.SHA256, a.WrittenAt.Unix(), nullStr(a.ManifestPath),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListArtifactsByRun devolve artifacts por run.
func (s *Store) ListArtifactsByRun(runID string) ([]Artifact, error) {
	rows, err := s.db.Query(`SELECT id, run_id, name, size, sha256, written_at, manifest_path FROM artifacts WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Artifact
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanArtifact(row rowScanner) (Artifact, error) {
	var a Artifact
	var written int64
	var manifest sql.NullString
	if err := row.Scan(&a.ID, &a.RunID, &a.Name, &a.Size, &a.SHA256, &written, &manifest); err != nil {
		return Artifact{}, err
	}
	a.WrittenAt = time.Unix(written, 0).UTC()
	if manifest.Valid {
		a.ManifestPath = manifest.String
	}
	return a, nil
}

// ErrNotFound é devolvido por Get quando o registro não existe.
var ErrNotFound = fmt.Errorf("storage: not found")

// nullStr devolve sql.NullString.
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullInt64 devolve sql.NullInt64.
func nullInt64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// nullTime devolve sql.NullInt64 (Unix).
func nullTime(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

// QuoteIdentifier escapa nome de tabela/coluna para SQL dinâmico.
// Whitelist alfanumérico + underscore; rejeita vazio.
func QuoteIdentifier(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("storage: identifier vazio")
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return "", fmt.Errorf("storage: identifier inválido: %s", name)
		}
	}
	return name, nil
}

// QuoteValue escapa valor para SQL literal (não é parametrização). Use
// com cautela; preferir `?` parametrizado.
func QuoteValue(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
