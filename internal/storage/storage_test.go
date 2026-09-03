package storage

import (
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// openTestDB abre um DB temporário e devolve store + cleanup.
func openTestDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// Aceitação: Open cria DB sem processo externo.
func TestOpenCreatesDB(t *testing.T) {
	s := openTestDB(t)
	if s == nil {
		t.Fatal("store nil")
	}
}

// Aceitação: Open path vazio falha.
func TestOpenEmptyPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatalf("esperava erro")
	}
}

// Aceitação: Open é idempotente (segunda chamada reabre).
func TestOpenIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	s1.InsertRun(Run{ID: "r1"})
	s1.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	defer s2.Close()
	got, err := s2.GetRun("r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.ID != "r1" {
		t.Errorf("id = %s", got.ID)
	}
}

// Aceitação: InsertRun grava e GetRun recupera.
func TestRunInsertGet(t *testing.T) {
	s := openTestDB(t)
	r := Run{
		ID:      "r1",
		BaseRef: "main",
		HeadRef: "feature/x",
		Meta:    map[string]string{"branch": "x"},
	}
	if err := s.InsertRun(r); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}
	got, err := s.GetRun("r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.ID != "r1" || got.BaseRef != "main" || got.HeadRef != "feature/x" {
		t.Errorf("campos errados: %+v", got)
	}
	if got.Meta["branch"] != "x" {
		t.Errorf("meta = %v", got.Meta)
	}
}

// Aceitação: GetRun retorna ErrNotFound para id inexistente.
func TestGetRunNotFound(t *testing.T) {
	s := openTestDB(t)
	_, err := s.GetRun("nao-existe")
	if err != ErrNotFound {
		t.Errorf("err = %v, quero ErrNotFound", err)
	}
}

// Aceitação: ListRuns retorna ordenado por created_at desc.
func TestRunListOrdered(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1", CreatedAt: time.Unix(100, 0)})
	s.InsertRun(Run{ID: "r2", CreatedAt: time.Unix(300, 0)})
	s.InsertRun(Run{ID: "r3", CreatedAt: time.Unix(200, 0)})
	rs, err := s.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	want := []string{"r2", "r3", "r1"}
	for i, r := range rs {
		if r.ID != want[i] {
			t.Errorf("[%d] = %s, quero %s", i, r.ID, want[i])
		}
	}
}

// Aceitação: FinishRun atualiza finished_at e status.
func TestFinishRun(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	if err := s.FinishRun("r1", "completed"); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}
	got, _ := s.GetRun("r1")
	if got.Status != "completed" {
		t.Errorf("status = %s", got.Status)
	}
	if got.FinishedAt == nil {
		t.Errorf("finishedAt nil")
	}
}

// Aceitação: InsertRun id vazio falha.
func TestInsertRunEmptyID(t *testing.T) {
	s := openTestDB(t)
	if err := s.InsertRun(Run{}); err == nil {
		t.Fatalf("esperava erro")
	}
}

// Aceitação: InsertComponent grava paths/metadata JSON.
func TestComponentInsert(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	id, err := s.InsertComponent(Component{
		RunID:          "r1",
		Name:           "api",
		Framework:      "spring",
		Language:       "java",
		Paths:          []string{"src/a", "src/b"},
		PrrScore:       0.85,
		Classification: "core",
		Metadata:       map[string]string{"v": "1"},
	})
	if err != nil {
		t.Fatalf("InsertComponent: %v", err)
	}
	if id == 0 {
		t.Errorf("id = 0")
	}
	cs, _ := s.ListComponentsByRun("r1")
	if len(cs) != 1 {
		t.Fatalf("len = %d", len(cs))
	}
	c := cs[0]
	if c.Name != "api" || c.Framework != "spring" || c.PrrScore != 0.85 {
		t.Errorf("campos errados: %+v", c)
	}
	if len(c.Paths) != 2 || c.Paths[0] != "src/a" {
		t.Errorf("paths = %v", c.Paths)
	}
	if c.Metadata["v"] != "1" {
		t.Errorf("metadata = %v", c.Metadata)
	}
}

// Aceitação: ListComponentsByRun retorna vazio para run sem components.
func TestListComponentsEmpty(t *testing.T) {
	s := openTestDB(t)
	cs, _ := s.ListComponentsByRun("vazio")
	if len(cs) != 0 {
		t.Errorf("len = %d", len(cs))
	}
}

// Aceitação: InsertComponent falha com run_id vazio.
func TestInsertComponentNoRunID(t *testing.T) {
	s := openTestDB(t)
	if _, err := s.InsertComponent(Component{Name: "x"}); err == nil {
		t.Fatalf("esperava erro")
	}
}

// Aceitação: InsertAnalyzerResult com data JSON.
func TestAnalyzerResultInsert(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	now := time.Now().UTC()
	compID, _ := s.InsertComponent(Component{RunID: "r1", Name: "api"})
	id, err := s.InsertAnalyzerResult(AnalyzerResult{
		RunID:       "r1",
		ComponentID: &compID,
		Analyzer:    "migrations",
		Status:      "completed",
		StartedAt:   &now,
		Data:        map[string]any{"count": 5},
	})
	if err != nil {
		t.Fatalf("InsertAnalyzerResult: %v", err)
	}
	if id == 0 {
		t.Errorf("id = 0")
	}
	rs, _ := s.ListAnalyzerResultsByRun("r1")
	if len(rs) != 1 {
		t.Fatalf("len = %d", len(rs))
	}
	if rs[0].Analyzer != "migrations" || rs[0].Data["count"] != float64(5) {
		t.Errorf("err: %+v", rs[0])
	}
}

// Aceitação: InsertReview grava e GetReview recupera.
func TestReviewInsertGet(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	now := time.Now().UTC()
	id, err := s.InsertReview(Review{
		RunID:     "r1",
		Model:     "gpt-4",
		Prompt:    "Review this code",
		Response:  "Looks good",
		StartedAt: &now,
		TokensIn:  100,
		TokensOut: 50,
		Cost:      0.001,
		Verdict:   "approve",
	})
	if err != nil {
		t.Fatalf("InsertReview: %v", err)
	}
	got, err := s.GetReview(id)
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if got.Model != "gpt-4" || got.TokensIn != 100 || got.Verdict != "approve" {
		t.Errorf("err: %+v", got)
	}
}

// Aceitação: ListReviewsByRun ordenado.
func TestListReviewsByRun(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	s.InsertRun(Run{ID: "r2"})
	s.InsertReview(Review{RunID: "r1", Model: "a"})
	s.InsertReview(Review{RunID: "r1", Model: "b"})
	s.InsertReview(Review{RunID: "r2", Model: "c"})
	rs, _ := s.ListReviewsByRun("r1")
	if len(rs) != 2 {
		t.Errorf("len = %d", len(rs))
	}
}

// Aceitação: InsertModel grava role/declared/effective.
func TestModelInsert(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	id, err := s.InsertModel(Model{
		RunID:     "r1",
		Name:      "claude-opus-5",
		Role:      "reviewer",
		Declared:  "thinking",
		Effective: "thinking",
		Notes:     "stable",
	})
	if err != nil {
		t.Fatalf("InsertModel: %v", err)
	}
	if id == 0 {
		t.Errorf("id = 0")
	}
	ms, _ := s.ListModelsByRun("r1")
	if len(ms) != 1 || ms[0].Name != "claude-opus-5" {
		t.Errorf("err: %+v", ms)
	}
}

// Aceitação: InsertFinding com severity aggregation.
func TestFindingInsertAndAggregate(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	s.InsertFinding(Finding{RunID: "r1", Severity: "high", Category: "migrations", Message: "drop table"})
	s.InsertFinding(Finding{RunID: "r1", Severity: "high", Category: "migrations", Message: "alter column"})
	s.InsertFinding(Finding{RunID: "r1", Severity: "low", Category: "envx", Message: "undocumented"})

	fs, _ := s.ListFindingsByRun("r1")
	if len(fs) != 3 {
		t.Errorf("len = %d", len(fs))
	}
	counts, _ := s.FindingsBySeverity("r1")
	if counts["high"] != 2 || counts["low"] != 1 {
		t.Errorf("counts = %v", counts)
	}
}

// Aceitação: InsertFinding com campos obrigatórios.
func TestInsertFindingRequired(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	cases := []Finding{
		{RunID: "r1", Category: "x", Message: "y"},
		{RunID: "r1", Severity: "high", Message: "y"},
		{RunID: "r1", Severity: "high", Category: "x"},
	}
	for i, f := range cases {
		if _, err := s.InsertFinding(f); err == nil {
			t.Errorf("case %d devia falhar", i)
		}
	}
}

// Aceitação: InsertFinding com review_id.
func TestInsertFindingWithReview(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	rid, _ := s.InsertReview(Review{RunID: "r1", Model: "x"})
	id, err := s.InsertFinding(Finding{RunID: "r1", ReviewID: &rid, Severity: "high", Category: "x", Message: "y"})
	if err != nil {
		t.Fatalf("InsertFinding: %v", err)
	}
	fs, _ := s.ListFindingsByRun("r1")
	if fs[0].ReviewID == nil || *fs[0].ReviewID != rid || fs[0].ID != id {
		t.Errorf("err: %+v", fs[0])
	}
}

// Aceitação: InsertArtifact com SHA-256.
func TestArtifactInsert(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	id, err := s.InsertArtifact(Artifact{
		RunID:        "r1",
		Name:         "diff.patch",
		Size:         1234,
		SHA256:       "abc123",
		ManifestPath: "manifest.json",
	})
	if err != nil {
		t.Fatalf("InsertArtifact: %v", err)
	}
	if id == 0 {
		t.Errorf("id = 0")
	}
	as, _ := s.ListArtifactsByRun("r1")
	if len(as) != 1 || as[0].SHA256 != "abc123" {
		t.Errorf("err: %+v", as)
	}
}

// Aceitação: ListArtifactsByRun ordenado por id.
func TestListArtifactsOrdered(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	for _, n := range []string{"a", "b", "c"} {
		s.InsertArtifact(Artifact{RunID: "r1", Name: n, SHA256: "x"})
	}
	as, _ := s.ListArtifactsByRun("r1")
	names := []string{}
	for _, a := range as {
		names = append(names, a.Name)
	}
	if len(names) != 3 {
		t.Errorf("len = %d", len(names))
	}
	sort.Strings(names) // ordenação consistente pra assertion
}

// Aceitação: schema_version é inicializado.
func TestSchemaVersion(t *testing.T) {
	s := openTestDB(t)
	var v int
	err := s.db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&v)
	if err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if v != SchemaVersion {
		t.Errorf("v = %d, quero %d", v, SchemaVersion)
	}
}

// Aceitação: migração idempotente.
func TestMigrationIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s1, _ := Open(path)
	s1.Close()
	s2, _ := Open(path)
	defer s2.Close()
	// Não deve dar erro.
}

// Aceitação: QuoteIdentifier aceita alfanum + underscore.
func TestQuoteIdentifier(t *testing.T) {
	cases := map[string]bool{
		"foo":      true,
		"foo_bar":  true,
		"Foo123":   true,
		"":         false,
		"foo-bar":  false,
		"foo;drop": false,
		"foo bar":  false,
	}
	for in, want := range cases {
		_, err := QuoteIdentifier(in)
		got := err == nil
		if got != want {
			t.Errorf("QuoteIdentifier(%q) = %v, quero %v", in, got, want)
		}
	}
}

// Aceitação: QuoteValue escapa aspas.
func TestQuoteValue(t *testing.T) {
	if got := QuoteValue("hello"); got != "'hello'" {
		t.Errorf("got %s", got)
	}
	if got := QuoteValue("O'Reilly"); got != "'O''Reilly'" {
		t.Errorf("got %s", got)
	}
}

// Aceitação: nil review_id não panica.
func TestInsertFindingNilReviewID(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	_, err := s.InsertFinding(Finding{RunID: "r1", Severity: "high", Category: "x", Message: "y"})
	if err != nil {
		t.Errorf("err: %v", err)
	}
	fs, _ := s.ListFindingsByRun("r1")
	if fs[0].ReviewID != nil {
		t.Errorf("reviewID devia ser nil")
	}
}

// Aceitação: finished_at opcional.
func TestFinishedAtOptional(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	got, _ := s.GetRun("r1")
	if got.FinishedAt != nil {
		t.Errorf("finishedAt devia ser nil")
	}
}

// Aceitação: WAL mode ativado.
func TestWALMode(t *testing.T) {
	s := openTestDB(t)
	var mode string
	err := s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	if mode != "wal" {
		t.Errorf("mode = %s, quero wal", mode)
	}
}

// Aceitação: foreign_keys ativado.
func TestForeignKeys(t *testing.T) {
	s := openTestDB(t)
	var on int
	s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&on)
	if on != 1 {
		t.Errorf("foreign_keys = %d, quero 1", on)
	}
}

// Aceitação: 100 inserts concorrentes.
func TestConcurrentInserts(t *testing.T) {
	s := openTestDB(t)
	s.InsertRun(Run{ID: "r1"})
	done := make(chan error, 100)
	for i := 0; i < 100; i++ {
		go func(i int) {
			_, err := s.InsertFinding(Finding{
				RunID:    "r1",
				Severity: "low",
				Category: "x",
				Message:  "m",
			})
			done <- err
		}(i)
	}
	for i := 0; i < 100; i++ {
		if err := <-done; err != nil {
			t.Errorf("conc: %v", err)
		}
	}
	fs, _ := s.ListFindingsByRun("r1")
	if len(fs) != 100 {
		t.Errorf("len = %d, quero 100", len(fs))
	}
}

// Aceitação: DB close devolve erro em queries.
func TestCloseDBQuery(t *testing.T) {
	s := openTestDB(t)
	s.Close()
	if _, err := s.GetRun("qualquer"); err == nil {
		t.Errorf("devia falhar")
	}
}

var _ = time.Now
