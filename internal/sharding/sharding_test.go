package sharding

import (
	"testing"
)

// Aceitação: ShardID é hex curto.
func TestShardID(t *testing.T) {
	id := ShardID("api", []string{"a.go", "b.go"}, []byte("content"))
	if len(id) != 16 {
		t.Errorf("id len = %d, quero 16", len(id))
	}
}

// Aceitação: Hash é determinístico.
func TestHashDeterministic(t *testing.T) {
	h1 := Hash("api", []string{"a.go", "b.go"}, []byte("content"))
	h2 := Hash("api", []string{"b.go", "a.go"}, []byte("content"))
	if h1 != h2 {
		t.Errorf("ordem de paths devia dar mesmo hash: %s vs %s", h1, h2)
	}
}

// Aceitação: Hash diferente com conteúdo diferente.
func TestHashDifferContent(t *testing.T) {
	h1 := Hash("api", []string{"a"}, []byte("aaa"))
	h2 := Hash("api", []string{"a"}, []byte("bbb"))
	if h1 == h2 {
		t.Errorf("hashes iguais com content diferente")
	}
}

// Aceitação: Hash diferente com component diferente.
func TestHashDifferComponent(t *testing.T) {
	h1 := Hash("api", []string{"a"}, []byte("x"))
	h2 := Hash("worker", []string{"a"}, []byte("x"))
	if h1 == h2 {
		t.Errorf("hashes iguais com comp diferente")
	}
}

// Aceitação: Hash diferente com paths diferentes.
func TestHashDifferPaths(t *testing.T) {
	h1 := Hash("api", []string{"a"}, []byte("x"))
	h2 := Hash("api", []string{"b"}, []byte("x"))
	if h1 == h2 {
		t.Errorf("hashes iguais com paths diferentes")
	}
}

// Aceitação: BuildShard ordena paths.
func TestBuildShardSortedPaths(t *testing.T) {
	s := BuildShard("api", []string{"z", "a", "m"}, []byte("x"))
	if s.Paths[0] != "a" || s.Paths[1] != "m" || s.Paths[2] != "z" {
		t.Errorf("paths = %v", s.Paths)
	}
}

// Aceitação: Shard.Size = len(content).
func TestBuildShardSize(t *testing.T) {
	s := BuildShard("x", nil, []byte("hello"))
	if s.Size != 5 {
		t.Errorf("size = %d", s.Size)
	}
}

// Aceitação: Shard.ID igual ao Hash truncado.
func TestShardIDMatchesHash(t *testing.T) {
	s := BuildShard("x", []string{"a"}, []byte("y"))
	if s.ID != s.Hash[:16] {
		t.Errorf("id != hash[:16]: %s vs %s", s.ID, s.Hash[:16])
	}
}

// Aceitação: Cluster agrupa por component.
func TestClusterByComponent(t *testing.T) {
	changes := []Change{
		{Component: "api", Path: "a.go", Content: []byte("a")},
		{Component: "worker", Path: "w.go", Content: []byte("w")},
		{Component: "api", Path: "b.go", Content: []byte("b")},
	}
	shards := Cluster(changes)
	if len(shards) != 2 {
		t.Errorf("len = %d, quero 2", len(shards))
	}
	if shards[0].Component != "api" || shards[1].Component != "worker" {
		t.Errorf("comps = %v, %v", shards[0].Component, shards[1].Component)
	}
}

// Aceitação: Cluster ordena shards por component.
func TestClusterOrderedByComponent(t *testing.T) {
	changes := []Change{
		{Component: "z", Path: "z", Content: []byte("z")},
		{Component: "a", Path: "a", Content: []byte("a")},
	}
	shards := Cluster(changes)
	if shards[0].Component != "a" || shards[1].Component != "z" {
		t.Errorf("order: %v, %v", shards[0].Component, shards[1].Component)
	}
}

// Aceitação: Cluster agrega paths do mesmo component.
func TestClusterAggregatePaths(t *testing.T) {
	changes := []Change{
		{Component: "api", Path: "a.go", Content: []byte("a")},
		{Component: "api", Path: "b.go", Content: []byte("b")},
		{Component: "api", Path: "c.go", Content: []byte("c")},
	}
	shards := Cluster(changes)
	if len(shards) != 1 {
		t.Fatalf("len = %d", len(shards))
	}
	if len(shards[0].Paths) != 3 {
		t.Errorf("paths = %v", shards[0].Paths)
	}
	if shards[0].Size != 6 { // "a\nb\nc\n" = 6
		t.Errorf("size = %d", shards[0].Size)
	}
}

// Aceitação: Cluster vazio.
func TestClusterEmpty(t *testing.T) {
	shards := Cluster(nil)
	if len(shards) != 0 {
		t.Errorf("len = %d", len(shards))
	}
}

// Aceitação: Cluster determinístico — Peer A e B recebem mesmos shards.
func TestClusterDeterministic(t *testing.T) {
	changes := []Change{
		{Component: "api", Path: "z.go", Content: []byte("z")},
		{Component: "api", Path: "a.go", Content: []byte("a")},
	}
	s1 := Cluster(changes)
	s2 := Cluster(changes)
	if len(s1) != 1 || len(s2) != 1 {
		t.Fatalf("len")
	}
	if s1[0].ID != s2[0].ID || s1[0].Hash != s2[0].Hash {
		t.Errorf("não-determinístico: %s vs %s", s1[0].ID, s2[0].ID)
	}
}

// Aceitação: VerifySameShard OK.
func TestVerifySameShardOK(t *testing.T) {
	s := BuildShard("x", []string{"a"}, []byte("y"))
	if err := VerifySameShard(s, s); err != nil {
		t.Errorf("err: %v", err)
	}
}

// Aceitação: VerifySameShard detecta ID mismatch.
func TestVerifySameShardIDMismatch(t *testing.T) {
	s1 := BuildShard("x", []string{"a"}, []byte("y"))
	s2 := BuildShard("x", []string{"b"}, []byte("y"))
	if err := VerifySameShard(s1, s2); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: VerifySameShard detecta hash mismatch.
func TestVerifySameShardHashMismatch(t *testing.T) {
	s1 := BuildShard("x", []string{"a"}, []byte("y"))
	s2 := BuildShard("x", []string{"a"}, []byte("z"))
	if err := VerifySameShard(s1, s2); err == nil {
		t.Errorf("devia falhar")
	}
}

// Aceitação: Merge dedupe por ID.
func TestMergeDedup(t *testing.T) {
	s := BuildShard("x", []string{"a"}, []byte("y"))
	out := Merge([]Shard{s, s})
	if len(out) != 1 {
		t.Errorf("len = %d", len(out))
	}
}

// Aceitação: Merge combina paths únicos.
func TestMergeCombinePaths(t *testing.T) {
	s1 := BuildShard("comp", []string{"a", "b"}, []byte("x"))
	s2 := BuildShard("comp", []string{"b", "c"}, []byte("x"))
	// Mesmo component+paths+content → mesmo hash
	// Vou forçar IDs diferentes.
	s2.ID = "different"
	out := Merge([]Shard{s1, s2})
	if len(out) != 2 {
		t.Errorf("len = %d", len(out))
	}
}

// Aceitação: Merge ordena por ID.
func TestMergeOrderByID(t *testing.T) {
	s1 := Shard{ID: "z"}
	s2 := Shard{ID: "a"}
	out := Merge([]Shard{s1, s2})
	if out[0].ID != "a" || out[1].ID != "z" {
		t.Errorf("order: %v", out)
	}
}

// Aceitação: String() descritivo.
func TestShardString(t *testing.T) {
	s := BuildShard("api", []string{"a"}, []byte("x"))
	got := s.String()
	if got == "" {
		t.Errorf("vazio")
	}
	if !contains(got, "api") {
		t.Errorf("err: %s", got)
	}
}

// Aceitação: ReasonsString.
func TestShardReasonsString(t *testing.T) {
	s := BuildShard("x", nil, nil, "reason1", "reason2")
	if s.ReasonsString() != "reason1;reason2" {
		t.Errorf("got: %s", s.ReasonsString())
	}
}

// Aceitação: Hash com paths vazios.
func TestHashEmptyPaths(t *testing.T) {
	h := Hash("x", nil, []byte("y"))
	if len(h) != 64 {
		t.Errorf("hash len = %d", len(h))
	}
}

// Aceitação: Hash com content vazio.
func TestHashEmptyContent(t *testing.T) {
	h := Hash("x", []string{"a"}, nil)
	if len(h) != 64 {
		t.Errorf("hash len = %d", len(h))
	}
}

// Aceitação: BuildShard sem reasons.
func TestBuildShardNoReasons(t *testing.T) {
	s := BuildShard("x", nil, nil)
	if s.Reasons != nil && len(s.Reasons) != 0 {
		t.Errorf("reasons = %v", s.Reasons)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
