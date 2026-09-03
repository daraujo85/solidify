// Package sharding — produção de shards estáveis para análise paralela.
//
// SAI-025: component → change cluster → shard. Peer A/B executam o
// mesmo sharding sobre o mesmo input e recebem shards com mesmo ID e
// hash, garantindo que a análise é particionável de forma reprodutível.
//
// Algoritmo:
//  1. Mudanças vêm como (component, paths, content).
//  2. Cluster: agrupa por component. Cada cluster vira um shard.
//  3. Hash: SHA-256(component + sorted(paths) + content).
//  4. ID: derivado do hash (hex curto).
package sharding

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Shard é uma unidade de análise paralela.
type Shard struct {
	ID        string   // hex curto, derivado do hash
	Component string   // component name
	Paths     []string // paths do cluster (ordenados)
	Hash      string   // SHA-256 hex do conteúdo canônico
	Size      int      // bytes do conteúdo
	Reasons   []string // motivos da clusterização (audit)
}

// Change é uma mudança unitária (de git diff).
type Change struct {
	Component string
	Path      string
	Content   []byte
}

// ShardID devolve o ID hex curto (16 chars) de um shard canônico.
func ShardID(component string, paths []string, content []byte) string {
	return Hash(component, paths, content)[:16]
}

// Hash devolve SHA-256 hex do conteúdo canônico (component + sorted
// paths + content).
func Hash(component string, paths []string, content []byte) string {
	h := sha256.New()
	h.Write([]byte(component))
	h.Write([]byte{0})
	sorted := append([]string{}, paths...)
	sort.Strings(sorted)
	for _, p := range sorted {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	h.Write([]byte{0})
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// BuildShard constrói um shard a partir de component + paths + content.
func BuildShard(component string, paths []string, content []byte, reasons ...string) Shard {
	sorted := append([]string{}, paths...)
	sort.Strings(sorted)
	hash := Hash(component, sorted, content)
	return Shard{
		ID:        hash[:16],
		Component: component,
		Paths:     sorted,
		Hash:      hash,
		Size:      len(content),
		Reasons:   append([]string{}, reasons...),
	}
}

// Cluster agrupa changes por component, devolvendo slices de Shard.
//
// Cada cluster (component) vira um shard. Paths são união dos paths
// das changes. Content é a concatenação ordenada (por path) dos
// conteúdos — peer A/B recebem o mesmo conteúdo porque a ordem é
// determinística.
func Cluster(changes []Change) []Shard {
	byComp := make(map[string][]Change)
	for _, c := range changes {
		byComp[c.Component] = append(byComp[c.Component], c)
	}
	comps := make([]string, 0, len(byComp))
	for c := range byComp {
		comps = append(comps, c)
	}
	sort.Strings(comps)
	var out []Shard
	for _, comp := range comps {
		cs := byComp[comp]
		sort.Slice(cs, func(i, j int) bool { return cs[i].Path < cs[j].Path })
		paths := make([]string, 0, len(cs))
		var content []byte
		for _, c := range cs {
			paths = append(paths, c.Path)
			content = append(content, c.Content...)
			content = append(content, '\n')
		}
		out = append(out, BuildShard(comp, paths, content, "component-cluster"))
	}
	return out
}

// Merge shards com mesmo ID. Útil para combinar shards de múltiplos
// peers (B peer review).
func Merge(shards []Shard) []Shard {
	byID := make(map[string]Shard)
	for _, s := range shards {
		if existing, ok := byID[s.ID]; ok {
			// Merge paths.
			seen := make(map[string]bool)
			for _, p := range existing.Paths {
				seen[p] = true
			}
			for _, p := range s.Paths {
				if !seen[p] {
					seen[p] = true
					existing.Paths = append(existing.Paths, p)
				}
			}
			sort.Strings(existing.Paths)
			// Mantém hash do primeiro peer (assume consistência).
			byID[s.ID] = existing
		} else {
			byID[s.ID] = s
		}
	}
	out := make([]Shard, 0, len(byID))
	for _, s := range byID {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// VerifySameShard devolve nil se s1 e s2 representam o mesmo shard
// (mesmo ID e hash). Caso contrário, devolve erro descritivo.
func VerifySameShard(s1, s2 Shard) error {
	if s1.ID != s2.ID {
		return fmt.Errorf("sharding: ID mismatch: %s vs %s", s1.ID, s2.ID)
	}
	if s1.Hash != s2.Hash {
		return fmt.Errorf("sharding: hash mismatch: %s vs %s", s1.Hash, s2.Hash)
	}
	return nil
}

// String devolve descrição legível.
func (s Shard) String() string {
	return fmt.Sprintf("Shard{id=%s,comp=%s,paths=%d,size=%d}", s.ID, s.Component, len(s.Paths), s.Size)
}

// ReasonsString devolve reasons joined por ";".
func (s Shard) ReasonsString() string {
	return strings.Join(s.Reasons, ";")
}
