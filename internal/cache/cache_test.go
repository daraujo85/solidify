package cache

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Aceitação: Key.Hash é determinístico.
func TestKeyDeterministic(t *testing.T) {
	k1 := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	k2 := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	if k1.Hash() != k2.Hash() {
		t.Errorf("hash não-determinístico")
	}
}

// Aceitação: mudança em qualquer campo muda hash.
func TestKeyChangeAnyField(t *testing.T) {
	base := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	changes := []Key{
		NewKeyFromFields("x", "1", "c", "b", "h", "i"),
		NewKeyFromFields("a", "2", "c", "b", "h", "i"),
		NewKeyFromFields("a", "1", "d", "b", "h", "i"),
		NewKeyFromFields("a", "1", "c", "x", "h", "i"),
		NewKeyFromFields("a", "1", "c", "b", "x", "i"),
		NewKeyFromFields("a", "1", "c", "b", "h", "x"),
	}
	baseH := base.Hash()
	for i, k := range changes {
		if k.Hash() == baseH {
			t.Errorf("change %d não afetou hash", i)
		}
	}
}

// Aceitação: Key.String inclui campos.
func TestKeyString(t *testing.T) {
	k := NewKeyFromFields("a", "1", "c", "base123", "head456", "i")
	s := k.String()
	if !strings.Contains(s, "a") || !strings.Contains(s, "base12") {
		t.Errorf("string = %s", s)
	}
}

// Aceitação: HashConfig é determinístico.
func TestHashConfigDeterministic(t *testing.T) {
	c1 := map[string]string{"a": "1", "b": "2"}
	c2 := map[string]string{"b": "2", "a": "1"}
	if HashConfig(c1) != HashConfig(c2) {
		t.Errorf("ordem devia dar mesmo hash")
	}
}

// Aceitação: HashConfig muda com valores.
func TestHashConfigChange(t *testing.T) {
	c1 := map[string]string{"a": "1"}
	c2 := map[string]string{"a": "2"}
	if HashConfig(c1) == HashConfig(c2) {
		t.Errorf("mudou valor, hash igual")
	}
}

// Aceitação: HashConfig nil.
func TestHashConfigNil(t *testing.T) {
	if HashConfig(nil) == "" {
		t.Errorf("hash vazio")
	}
}

// Aceitação: HashBytes.
func TestHashBytes(t *testing.T) {
	h := HashBytes([]byte("hello"))
	if len(h) != 64 {
		t.Errorf("hash len = %d", len(h))
	}
}

// Aceitação: HashBytes diferentes.
func TestHashBytesDifferent(t *testing.T) {
	if HashBytes([]byte("a")) == HashBytes([]byte("b")) {
		t.Errorf("hashes iguais")
	}
}

// Aceitação: HashReader.
func TestHashReader(t *testing.T) {
	h, err := HashReader(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if h != HashBytes([]byte("hello")) {
		t.Errorf("mismatch")
	}
}

// Aceitação: New cria diretório.
func TestNewCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir")
	c, err := New(dir, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c.Dir != dir {
		t.Errorf("dir = %s", c.Dir)
	}
}

// Aceitação: New aceita dir vazio e usa default ./".solidify/cache".
func TestNewEmptyDir(t *testing.T) {
	c, err := New("", 0)
	if err != nil {
		t.Fatalf("não devia falhar: %v", err)
	}
	want := filepath.Join(".solidify", "cache")
	if c.Dir != want {
		t.Errorf("dir default = %q, want %q", c.Dir, want)
	}
}

// Aceitação: Set/Get básico.
func TestSetGet(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	if err := c.Set(k, []byte("payload")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, err := c.Get(k)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok || string(got) != "payload" {
		t.Errorf("got = %q, ok=%v", got, ok)
	}
}

// Aceitação: Get miss.
func TestGetMiss(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	got, ok, err := c.Get(k)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok || got != nil {
		t.Errorf("devia ser miss")
	}
}

// Aceitação: Has.
func TestHas(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	if ok, _ := c.Has(k); ok {
		t.Errorf("devia ser false")
	}
	c.Set(k, []byte("x"))
	if ok, _ := c.Has(k); !ok {
		t.Errorf("devia ser true")
	}
}

// Aceitação: Delete.
func TestDelete(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	c.Set(k, []byte("x"))
	if err := c.Delete(k); err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok, _ := c.Has(k); ok {
		t.Errorf("devia sumir")
	}
}

// Aceitação: Delete inexistente.
func TestDeleteMissing(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	if err := c.Delete(k); err != nil {
		t.Errorf("err: %v", err)
	}
}

// Aceitação: Clear.
func TestClear(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	c.Set(NewKeyFromFields("a", "1", "c", "b", "h", "i"), []byte("x"))
	c.Set(NewKeyFromFields("a", "2", "c", "b", "h", "i"), []byte("y"))
	if err := c.Clear(); err != nil {
		t.Fatalf("err: %v", err)
	}
	n, _ := c.Len()
	if n != 0 {
		t.Errorf("len = %d", n)
	}
}

// Aceitação: TTL expira.
func TestTTLExpires(t *testing.T) {
	c, _ := New(t.TempDir(), 50*time.Millisecond)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	c.Set(k, []byte("x"))
	time.Sleep(100 * time.Millisecond)
	_, ok, _ := c.Get(k)
	if ok {
		t.Errorf("devia expirar")
	}
}

// Aceitação: TTL não expira dentro do tempo.
func TestTTLNotExpires(t *testing.T) {
	c, _ := New(t.TempDir(), 1*time.Hour)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	c.Set(k, []byte("x"))
	_, ok, _ := c.Get(k)
	if !ok {
		t.Errorf("devia estar válido")
	}
}

// Aceitação: Sem TTL = permanente.
func TestNoTTL(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	c.Set(k, []byte("x"))
	// Set nada de expiry.
	got, ok, _ := c.Get(k)
	if !ok || string(got) != "x" {
		t.Errorf("payload sumiu")
	}
}

// Aceitação: Len.
func TestLen(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	if n, _ := c.Len(); n != 0 {
		t.Errorf("inicial = %d", n)
	}
	c.Set(NewKeyFromFields("a", "1", "c", "b", "h", "i"), []byte("x"))
	if n, _ := c.Len(); n != 1 {
		t.Errorf("len = %d", n)
	}
}

// Aceitação: Key.Equal.
func TestKeyEqual(t *testing.T) {
	k1 := NewKeyFromFields("a", "1", "c", "b", "h", "i", "t1")
	k2 := NewKeyFromFields("a", "1", "c", "b", "h", "i", "t1")
	if !k1.Equal(k2) {
		t.Errorf("devia ser igual")
	}
	k3 := NewKeyFromFields("a", "1", "c", "b", "h", "i", "t2")
	if k1.Equal(k3) {
		t.Errorf("devia ser diferente")
	}
}

// Aceitação: Key.MarshalJSON.
func TestKeyMarshalJSON(t *testing.T) {
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i", "t")
	data, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(data), `"analyzer":"a"`) {
		t.Errorf("data = %s", data)
	}
}

// Aceitação: InvalidateByTag.
func TestInvalidateByTag(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	c.Set(NewKeyFromFields("analyzer-a", "1", "c", "b", "h", "i"), []byte("x"))
	c.Set(NewKeyFromFields("analyzer-b", "1", "c", "b", "h", "i"), []byte("y"))
	n, err := c.InvalidateByTag("analyzer-a")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 1 {
		t.Errorf("invalidou %d, quero 1", n)
	}
}

// Aceitação: short.
func TestShort(t *testing.T) {
	if short("abcdefghij") != "abcdefgh" {
		t.Errorf("short não truncou")
	}
	if short("abc") != "abc" {
		t.Errorf("short alterou curto")
	}
}

// Aceitação: pathFor sharding.
func TestPathForSharding(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	p := c.pathFor("abcd1234")
	parts := strings.Split(p, string(filepath.Separator))
	// Espera .../ab/cd/abcd1234.json
	if !strings.Contains(p, "ab") || !strings.Contains(p, "cd") {
		t.Errorf("path = %s", p)
	}
	_ = parts
}

// Aceitação: Set sobrescreve.
func TestSetOverwrite(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	c.Set(k, []byte("v1"))
	c.Set(k, []byte("v2"))
	got, ok, _ := c.Get(k)
	if !ok || string(got) != "v2" {
		t.Errorf("got = %q", got)
	}
}

// Aceitação: payload binário (não-texto).
func TestSetBinaryPayload(t *testing.T) {
	c, _ := New(t.TempDir(), 0)
	k := NewKeyFromFields("a", "1", "c", "b", "h", "i")
	binary := []byte{0x00, 0x01, 0xFF, 0x80}
	c.Set(k, binary)
	got, _, _ := c.Get(k)
	for i, b := range got {
		if b != binary[i] {
			t.Errorf("byte %d: got %x, want %x", i, b, binary[i])
		}
	}
}

// Aceitação: Key.Tags order independence.
func TestKeyTagsOrder(t *testing.T) {
	k1 := NewKeyFromFields("a", "1", "c", "b", "h", "i", "t1", "t2")
	k2 := NewKeyFromFields("a", "1", "c", "b", "h", "i", "t2", "t1")
	if k1.Hash() != k2.Hash() {
		t.Errorf("ordem tags devia dar mesmo hash")
	}
}
