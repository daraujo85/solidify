// Peer review record migration v1 → v2 (SAI-122).
//
// v1: payload canônico em `Notes` (JSON-string).
// v2: payload canônico em `CanonicalPayload` (map nativo).
//
// MigrateV1ToV2 lê cada record do store, parseia Notes como JSON
// se schema == "1" e schema ainda não migrado, popula
// CanonicalPayload, limpa Notes, bump schema → "2", recomputa
// ContentHash. Records que já são v2 ou que não parseiam são pulados
// (best-effort).
package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MigrateResult relatório de migração.
type MigrateResult struct {
	TotalInStore int      `json:"total_in_store"`
	Scanned      int      `json:"scanned"`
	Migrated     int      `json:"migrated"`
	SkippedV2    int      `json:"skipped_v2"`
	SkippedBad   int      `json:"skipped_bad_json"`
	Errors       []string `json:"errors,omitempty"`
}

// MigrateOptions controla comportamento.
type MigrateOptions struct {
	DryRun bool   // não escreve; só reporta
	Actor  string // filtra por actor ("peer_a", "peer_b", "" = todos)
}

// MigrateV1ToV2 itera store e migra records v1 → v2 in-place.
// `opts.Actor` filtra (vazio = todos); `opts.DryRun` não persiste.
// Scanned conta apenas records pós-filtro (actor); TotalInStore
// sempre reflete o tamanho do diretório.
func MigrateV1ToV2(store *PeerReviewStore, opts MigrateOptions) (*MigrateResult, error) {
	if store == nil {
		return nil, errors.New("migrate: store nil")
	}
	records, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("migrate: list: %w", err)
	}
	res := &MigrateResult{TotalInStore: len(records)}
	for _, rec := range records {
		if rec == nil {
			continue
		}
		if opts.Actor != "" && rec.Actor != opts.Actor {
			continue
		}
		res.Scanned++
		if rec.Schema != "1" {
			res.SkippedV2++
			continue
		}
		if rec.Notes == "" {
			// v1 sem Notes = payload canônico faltando; pula
			res.SkippedBad++
			continue
		}
		var payload map[string]any
		if jerr := json.Unmarshal([]byte(rec.Notes), &payload); jerr != nil {
			res.SkippedBad++
			continue
		}
		if !opts.DryRun {
			// Bump schema, popula CanonicalPayload, limpa Notes,
			// recomputa ContentHash (canonical v2 difere de v1).
			rec.Schema = PeerReviewSchemaVersion // "2"
			rec.CanonicalPayload = payload
			rec.Notes = ""
			// Recompõe input equivalente pra reusar ComputeContentHash
			equiv := &SubmitPeerReviewInput{
				RunID:            rec.RunID,
				Actor:            rec.Actor,
				Schema:           rec.Schema,
				EvidenceHash:     rec.EvidenceHash,
				Verdict:          rec.Verdict,
				Findings:         rec.Findings,
				CanonicalPayload: rec.CanonicalPayload,
			}
			rec.ContentHash = ComputeContentHash(equiv)
			if serr := store.Save(rec); serr != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: save: %v", rec.ReviewID, serr))
				continue
			}
		}
		res.Migrated++
	}
	return res, nil
}
