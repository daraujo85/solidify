// Package cache — cache determinístico por hash de inputs.
//
// SAI-029: chave = SHA-256(analyzer|version|config|base|head|input).
// Inputs idênticos → mesma chave → cache hit. Mudou qualquer um
// → chave diferente → cache miss. Storage filesystem com TTL.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Key — componentes da chave de cache. Qualquer mudança invalida.
type Key struct {
	Analyzer     string   `json:"analyzer"`
	Version      string   `json:"version"`
	ConfigHash   string   `json:"config_hash"`   // hash da config do analyzer
	Base         string   `json:"base"`          // git base SHA
	Head         string   `json:"head"`          // git head SHA
	MergeBaseSHA string   `json:"merge_base_sha"` // git merge-base(base, head) SHA
	DiffStrategy string   `json:"diff_strategy"`  // como o diff foi calculado
	InputHash    string   `json:"input_hash"`     // hash do input (shard, diff, etc)
	Tags         []string `json:"tags,omitempty"` // tags extras (stack, lang)
}

// Hash devolve SHA-256 hex da representação canônica.
func (k Key) Hash() string {
	h := sha256.New()
	enc := jsonCanonical(k)
	h.Write(enc)
	return hex.EncodeToString(h.Sum(nil))
}

// String devolve key curto pra logs/debug.
func (k Key) String() string {
	return fmt.Sprintf("Key{a=%s,v=%s,b=%s,h=%s}", k.Analyzer, k.Version, short(k.Base), short(k.Head))
}

// Short label para SHAs.
func short(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// jsonCanonical serializa em ordem determinística.
func jsonCanonical(k Key) []byte {
	// Tags sorted.
	tags := append([]string{}, k.Tags...)
	sort.Strings(tags)
	// Manual canonical order.
	parts := []string{
		"analyzer=", k.Analyzer,
		"\nversion=", k.Version,
		"\nconfig_hash=", k.ConfigHash,
		"\nbase=", k.Base,
		"\nhead=", k.Head,
		"\nmerge_base_sha=", k.MergeBaseSHA,
		"\ndiff_strategy=", k.DiffStrategy,
		"\ninput_hash=", k.InputHash,
		"\ntags=", strings.Join(tags, ","),
	}
	return []byte(strings.Join(parts, ""))
}

// HashConfig devolve hash de uma config map[string]string. Aceita
// nil. Resultado é determinístico (chaves ordenadas).
func HashConfig(config map[string]string) string {
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(config[k]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// HashBytes devolve SHA-256 hex.
func HashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// HashReader consome reader e devolve SHA-256 hex (não carrega tudo).
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Entry metadata salvo junto com payload.
type Entry struct {
	Key       string    `json:"key"`
	Created   time.Time `json:"created"`
	Expires   time.Time `json:"expires,omitempty"`
	Payload   []byte    `json:"payload"`
	Analyzer  string    `json:"analyzer"`
	Version   string    `json:"version"`
	InputHash string    `json:"input_hash"`
}

// Cache filesystem-based.
type Cache struct {
	Dir string
	TTL time.Duration // 0 = sem expiry

	mu sync.Mutex
}

// New cria cache (mkdir -p Dir). Se dir == "", usa ./".solidify/cache".
func New(dir string, ttl time.Duration) (*Cache, error) {
	if dir == "" {
		dir = filepath.Join(".solidify", "cache")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("cache: mkdir: %w", err)
	}
	return &Cache{Dir: dir, TTL: ttl}, nil
}

// pathFor devolve path do arquivo de cache.
func (c *Cache) pathFor(key string) string {
	// Sharding: 2 níveis para não ter milhares em um único dir.
	if len(key) < 4 {
		return filepath.Join(c.Dir, key+".json")
	}
	return filepath.Join(c.Dir, key[:2], key[2:4], key+".json")
}

// Get devolve payload (nil se miss). Erro só em falha de I/O.
func (c *Cache) Get(k Key) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.pathFor(k.Hash())
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, false, fmt.Errorf("cache: unmarshal: %w", err)
	}
	// Verifica TTL.
	if !e.Expires.IsZero() && time.Now().After(e.Expires) {
		// Expired — remove best-effort.
		_ = os.Remove(p)
		return nil, false, nil
	}
	// Verifica key match (anti-collision paranoia).
	if e.Key != k.Hash() {
		return nil, false, fmt.Errorf("cache: key mismatch")
	}
	return e.Payload, true, nil
}

// Set grava payload.
func (c *Cache) Set(k Key, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.pathFor(k.Hash())
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("cache: mkdir: %w", err)
	}
	e := Entry{
		Key:       k.Hash(),
		Created:   time.Now(),
		Payload:   payload,
		Analyzer:  k.Analyzer,
		Version:   k.Version,
		InputHash: k.InputHash,
	}
	if c.TTL > 0 {
		e.Expires = e.Created.Add(c.TTL)
	}
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("cache: marshal: %w", err)
	}
	// Atomic write.
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("cache: write tmp: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("cache: rename: %w", err)
	}
	return nil
}

// Delete remove entrada.
func (c *Cache) Delete(k Key) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.pathFor(k.Hash())
	err := os.Remove(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Clear remove todas as entradas (best-effort).
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return os.RemoveAll(c.Dir)
}

// Has devolve true se há entry (sem carregar payload).
func (c *Cache) Has(k Key) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.pathFor(k.Hash())
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, nil
	}
	return true, nil
}

// Len devolve nº aproximado de entries.
func (c *Cache) Len() (int, error) {
	count := 0
	err := filepath.Walk(c.Dir, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info != nil && !info.IsDir() && filepath.Ext(info.Name()) == ".json" {
			count++
		}
		return nil
	})
	return count, err
}

// InvalidateByTag remove entries cuja key.Tag inclui tag.
func (c *Cache) InvalidateByTag(tag string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	err := filepath.Walk(c.Dir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info == nil || info.IsDir() || filepath.Ext(info.Name()) != ".json" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		var e Entry
		if err := json.Unmarshal(data, &e); err != nil {
			return nil
		}
		if e.Analyzer == tag {
			if err := os.Remove(p); err == nil {
				count++
			}
		}
		return nil
	})
	return count, err
}

// NewKeyFromFields é um helper pra construir Key rapidamente.
func NewKeyFromFields(analyzer, version, configHash, base, head, inputHash string, tags ...string) Key {
	return Key{
		Analyzer:   analyzer,
		Version:    version,
		ConfigHash: configHash,
		Base:       base,
		Head:       head,
		InputHash:  inputHash,
		Tags:       tags,
	}
}

// Equal devolve true se chaves são idênticas.
func (k Key) Equal(other Key) bool {
	return k.Analyzer == other.Analyzer &&
		k.Version == other.Version &&
		k.ConfigHash == other.ConfigHash &&
		k.Base == other.Base &&
		k.Head == other.Head &&
		k.MergeBaseSHA == other.MergeBaseSHA &&
		k.DiffStrategy == other.DiffStrategy &&
		k.InputHash == other.InputHash &&
		equalStrings(k.Tags, other.Tags)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// MarshalJSON canônico.
func (k Key) MarshalJSON() ([]byte, error) {
	type alias Key
	return json.Marshal(alias(k))
}
