package app

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/gitx"
	"github.com/diegoaraujo/solidify/internal/migrations"
	"github.com/diegoaraujo/solidify/internal/report"
)

// buildCommits popula report.GitInfo.Commits a partir de gitx.Log — antes
// deste wiring, bInput.Git.Commits nunca era atribuído em produção (única
// fonte de commits não-vazios era a fixture estática). Usa
// Conventional.Type diretamente (não internal/classification.ClassifyCommit,
// que responde outra pergunta — categorização por path — e não é o que
// mapDeliveries em mapping.js lê).
func buildCommits(dir, rng string) ([]report.Commit, error) {
	commits, err := gitx.Log(dir, rng, gitx.LogOpts{})
	if err != nil {
		return nil, err
	}
	out := make([]report.Commit, 0, len(commits))
	for _, c := range commits {
		rc := report.Commit{
			SHA:      c.SHA,
			ShortSHA: shortSHA(c.SHA),
			Subject:  c.Subject,
			Type:     c.Conventional.Type,
			Breaking: c.Conventional.Breaking,
			Issues:   c.IssueKeys,
		}
		if c.Conventional.Scope != "" {
			scope := c.Conventional.Scope
			rc.Scope = &scope
		}
		out = append(out, rc)
	}
	return out, nil
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// buildChangedFiles mapeia gitx.Changes (git diff --raw + --numstat) pro
// shape de report.ChangedFile. Antes deste wiring, bInput.Git.ChangedFiles
// nunca era atribuído em produção — mesma classe de gap de buildCommits.
func buildChangedFiles(changes []gitx.FileChange) []report.ChangedFile {
	out := make([]report.ChangedFile, 0, len(changes))
	for _, c := range changes {
		cf := report.ChangedFile{
			Path:   c.Path,
			Status: c.Status.String(),
			Binary: c.IsBinary,
		}
		if c.OldPath != "" {
			old := c.OldPath
			cf.OldPath = &old
		}
		if !c.IsBinary {
			add, del := c.Add, c.Del
			cf.AddedLines = &add
			cf.DeletedLines = &del
		}
		out = append(out, cf)
	}
	return out
}

// changeKindToMigrationStatus espelha gitx.ChangeKind pro enum de
// internal/migrations (mesmos 4 valores textuais: added/modified/deleted/
// renamed). Copied/TypeChanged não tem equivalente em migrations.ChangeStatus
// — tratamos como "modified" (mudança de conteúdo/metadata, não é rollback
// nem nova migration).
func changeKindToMigrationStatus(k gitx.ChangeKind) migrations.ChangeStatus {
	switch k {
	case gitx.ChangeAdded:
		return migrations.Added
	case gitx.ChangeDeleted:
		return migrations.Deleted
	case gitx.ChangeRenamed:
		return migrations.Renamed
	default:
		return migrations.Modified
	}
}

// rollbackSiblingPattern casa nomes de migration com convenção de rollback
// bem definida (Flyway V<version>__ -> U<version>__, ou sufixo/prefixo
// up/down). Fora desse conjunto, não há convenção confiável pra procurar
// sibling — RollbackPresent fica nil (desconhecido, nunca fabricado).
var rollbackSiblingPattern = regexp.MustCompile(`(?i)^(.*/)?V([0-9._]+)(__.*)\.sql$`)

// findRollbackSibling procura, dentro do próprio diff (changedSet), um
// arquivo de rollback pra uma migration Flyway recém-adicionada. Migration
// "modified"/"deleted", ou sem convenção reconhecida, devolve checked=false
// (não avaliamos: sibling pode existir no repo sem aparecer neste diff).
func findRollbackSibling(path string, status migrations.ChangeStatus, changedSet map[string]bool) (rollback string, checked bool) {
	if status != migrations.Added {
		return "", false
	}
	m := rollbackSiblingPattern.FindStringSubmatch(path)
	if m == nil {
		// convenção genérica up/down (ex.: 0032_add_x.up.sql / .down.sql).
		if strings.Contains(strings.ToLower(path), ".up.sql") {
			candidate := strings.Replace(path, path[strings.LastIndex(path, ".up.sql"):], ".down.sql", 1)
			if changedSet[candidate] {
				return candidate, true
			}
			return "", true
		}
		return "", false
	}
	candidate := m[1] + "U" + m[2] + m[3] + ".sql"
	if changedSet[candidate] {
		return candidate, true
	}
	return "", true
}

// buildMigrations detecta migrations no diff (internal/migrations.Detect),
// roda o parser SQL só quando o arquivo é .sql (única fonte que Parse sabe
// ler — frameworks com migration em .py/.rb/.php/.cs/.js ficam sem
// Operations, sem inventar parsing que não existe) e Assess pra risco +
// Impacto (migrations.ImpactForFindings, já testado). Sem wiring anterior:
// bInput.Migrations nunca era atribuído em produção.
func buildMigrations(dir string, changes []gitx.FileChange, cfg config.Detectors) []report.Migration {
	changedSet := make(map[string]bool, len(changes))
	for _, c := range changes {
		changedSet[c.Path] = true
	}

	dms := make([]migrations.DetectedMigration, 0, len(changes))
	statusByPath := make(map[string]migrations.ChangeStatus, len(changes))
	for _, c := range changes {
		st := changeKindToMigrationStatus(c.Status)
		statusByPath[c.Path] = st
		rollback, _ := findRollbackSibling(c.Path, st, changedSet)
		dms = append(dms, migrations.DetectedMigration{Path: c.Path, Status: st, Rollback: rollback})
	}

	detected := migrations.Detect(dms, cfg.MigrationPaths)
	out := make([]report.Migration, 0, len(detected))
	for _, m := range detected {
		var ops []migrations.Operation
		if strings.EqualFold(filepath.Ext(m.Path), ".sql") {
			if raw, err := os.ReadFile(filepath.Join(dir, m.Path)); err == nil {
				ops = migrations.Parse(string(raw))
			}
		}
		findings := migrations.Assess(m, ops)
		impacto := migrations.ImpactForFindings(findings)

		rm := report.Migration{
			ID:        migrationID(m.Path),
			Path:      m.Path,
			Framework: string(m.Framework),
			Status:    string(m.Status),
			Risk:      worstRiskLower(findings),
			Impacto:   string(impacto),
		}
		// RollbackPresent só é setado quando avaliamos convenção
		// reconhecida (ver findRollbackSibling) — do contrário fica nil
		// (desconhecido), nunca um "false" fabricado sobre migration cujo
		// rollback pode existir fora deste diff.
		if _, checked := findRollbackSibling(m.Path, statusByPath[m.Path], changedSet); checked {
			present := m.HasRollback
			rm.RollbackPresent = &present
		}
		out = append(out, rm)
	}
	return out
}

// envVarLinePattern casa "NOME=valor" numa linha de diff (prefixo +/- já
// removido) — mesma convenção de dotenv usada pelos 3 arquivos em
// cfg.Detectors.EnvDocumentationFiles.
var envVarLinePattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

// likelySecretEnvName heurística por nome (mesmas keywords que qualquer
// scanner de secret usa) — nunca abre o valor real (arquivos de doc são
// .example/.sample/.template, não .env de verdade; mesmo assim não expomos
// o RHS em nenhum campo do report).
var likelySecretEnvName = regexp.MustCompile(`(?i)(SECRET|TOKEN|PASSWORD|PWD|CREDENTIAL|API_KEY|PRIVATE_KEY)`)

// buildEnvChanges detecta variáveis de ambiente adicionadas/removidas nos
// arquivos de documentação (.env.example etc., cfg.EnvDocumentationFiles) —
// única fonte confiável de "isso é uma env var" sem parsear runtime de N
// linguagens. Documented=true sempre (viemos do próprio arquivo de doc);
// Components/References vêm de outros arquivos do MESMO diff que citam o
// nome — sinal real, não fabricado, mas incompleto por natureza (não
// escaneia o repo inteiro). Sem hunks nesses arquivos -> slice vazia.
// ponytail: Required/DeploymentAction sempre nil/"" (sem heurística
// confiável ainda); grep incremental fora do diff ficaria caro em repos
// grandes — upgrade quando houver sinal de uso real que justifique o custo.
func buildEnvChanges(diffFiles []gitx.DiffFile, cfg config.Detectors) []report.EnvChange {
	docSet := make(map[string]bool, len(cfg.EnvDocumentationFiles))
	for _, f := range cfg.EnvDocumentationFiles {
		docSet[f] = true
	}

	type state struct {
		added, removed bool
		defaultVal     string
	}
	found := make(map[string]*state)
	order := []string{}

	for _, df := range diffFiles {
		if !docSet[filepath.Base(df.Path)] {
			continue
		}
		for _, h := range df.Hunks {
			for _, line := range strings.Split(h.Content, "\n") {
				if len(line) < 2 {
					continue
				}
				sign, rest := line[0], line[1:]
				if sign != '+' && sign != '-' {
					continue
				}
				m := envVarLinePattern.FindStringSubmatch(rest)
				if m == nil {
					continue
				}
				name := m[1]
				st, ok := found[name]
				if !ok {
					st = &state{}
					found[name] = st
					order = append(order, name)
				}
				if sign == '+' {
					st.added = true
					st.defaultVal = m[2]
				} else {
					st.removed = true
				}
			}
		}
	}
	if len(found) == 0 {
		return []report.EnvChange{}
	}

	out := make([]report.EnvChange, 0, len(found))
	for _, name := range order {
		st := found[name]
		status := "modified"
		switch {
		case st.added && !st.removed:
			status = "added"
		case st.removed && !st.added:
			status = "removed"
		}
		var components []string
		var refs []map[string]any
		for _, df := range diffFiles {
			if docSet[filepath.Base(df.Path)] {
				continue
			}
			for _, h := range df.Hunks {
				if strings.Contains(h.Content, name) {
					components = append(components, df.Path)
					refs = append(refs, map[string]any{"file": df.Path})
					break
				}
			}
		}
		ec := report.EnvChange{
			Name:         name,
			Status:       status,
			Components:   components,
			Documented:   true,
			LikelySecret: likelySecretEnvName.MatchString(name),
			References:   refs,
		}
		if st.added {
			hasDefault := strings.TrimSpace(st.defaultVal) != ""
			ec.HasDefault = &hasDefault
		}
		out = append(out, ec)
	}
	return out
}

// migrationID deriva um ID estável e curto do path (fnv32, hex) — só
// precisa ser único e determinístico entre runs; sem esquema de
// numeração de negócio (isso é papel do framework de migration, não
// nosso). ponytail: se o contrato exigir IDs sequenciais legíveis
// (m-0001, m-0002...), trocar por índice na lista ordenada por path.
func migrationID(path string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(path))
	return fmt.Sprintf("m-%08x", h.Sum32())
}

// worstRiskLower agrega os findings pelo pior RiskLevel e devolve em
// minúsculo (mesma convenção da fixture: "low"/"moderate"/"high"/
// "critical"). Sem findings -> "" (nunca fabrica "low" como default).
func worstRiskLower(findings []migrations.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	rank := map[migrations.RiskLevel]int{migrations.RiskLow: 0, migrations.RiskModerate: 1, migrations.RiskHigh: 2, migrations.RiskCritical: 3}
	worst := migrations.RiskLow
	for _, f := range findings {
		if rank[f.Level] > rank[worst] {
			worst = f.Level
		}
	}
	return strings.ToLower(string(worst))
}
