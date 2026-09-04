// Package migrations — risk engine.
//
// Implementa os sinais de risco de ARCHITECTURE.md §10.4:
//
//	DROP TABLE/COLUMN/INDEX;
//	redução de tamanho/tipo potencialmente incompatível;
//	NOT NULL sem default/backfill aparente;
//	rename que possa quebrar consumidores;
//	update/delete massivo;
//	criação de índice potencialmente bloqueante;
//	migration sem rollback quando a stack normalmente o suporta;
//	ordem dependente (heurística básica);
//	migration editada depois de já existir na base.
//
// Não executa nada destrutivo — só marca para revisão. Conservador:
// se ambíguo, não marca.
package migrations

import (
	"regexp"
	"strings"
)

// RiskLevel é a severidade de um achado de risco.
type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskModerate RiskLevel = "MODERATE"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// Finding é um sinal de risco detectado em uma migration.
type Finding struct {
	Code      string    // ex: "DROP_TABLE", "NOT_NULL_NO_DEFAULT"
	Level     RiskLevel // LOW/MODERATE/HIGH/CRITICAL
	Message   string    // descrição legível
	Migration string    // path da migration
	Operation string    // tipo da operação que originou (ou vazio)
	Object    string    // tabela/view/index/coluna afetada
	Detail    string    // info extra (ex: "WHERE ausente")
}

// Assess devolve os findings de risco para uma migration dado o conjunto
// de operações detectadas pelo parser. Conservador: só emite finding
// quando tem certeza; ambíguo = silêncio.
func Assess(m Migration, ops []Operation) []Finding {
	var out []Finding
	// 1. Regras baseadas nas operações detectadas.
	for _, op := range ops {
		out = append(out, assessOp(m, op)...)
	}
	// 2. Regras baseadas em análise textual do Raw (NOT NULL sem default,
	//    CREATE INDEX sem CONCURRENTLY, UPDATE/DELETE sem WHERE).
	for _, op := range ops {
		out = append(out, assessRaw(m, op)...)
	}
	// 3. Regras baseadas em metadata (rollback ausente).
	out = append(out, assessMetadata(m)...)
	return out
}

// assessOp avalia regras puramente estruturais (sem olhar o Raw).
func assessOp(m Migration, op Operation) []Finding {
	var out []Finding
	switch op.Type {
	case OpDrop:
		// DROP TABLE / VIEW / TYPE / SCHEMA etc.
		if op.Detail != "" && op.Detail != "INDEX" {
			out = append(out, Finding{
				Code:      "DROP_OBJECT",
				Level:     riskForDropObject(op.Detail),
				Message:   "DROP destrutivo de " + op.Detail + " " + op.Object + " — reverter só via backup.",
				Migration: m.Path, Operation: string(op.Type),
				Object: op.Object, Detail: op.Detail,
			})
		}
	case OpAdd:
		// ADD COLUMN com NOT NULL sem default é capturado em assessRaw.
		// Aqui só marcamos como "schema mutation".
		out = append(out, Finding{
			Code:      "ADD_COLUMN",
			Level:     RiskLow,
			Message:   "ADD COLUMN em " + op.Object + "." + op.Detail + " — rodar em horário de baixo tráfego se a tabela for grande.",
			Migration: m.Path, Operation: string(op.Type),
			Object: op.Object, Detail: op.Detail,
		})
	case OpRename:
		out = append(out, Finding{
			Code:      "RENAME_BREAKING",
			Level:     RiskModerate,
			Message:   "RENAME de " + op.Object + " (" + op.Detail + ") — pode quebrar consumidores que dependem do nome antigo.",
			Migration: m.Path, Operation: string(op.Type),
			Object: op.Object, Detail: op.Detail,
		})
	case OpDropIndex:
		out = append(out, Finding{
			Code:      "DROP_INDEX",
			Level:     RiskModerate,
			Message:   "DROP INDEX " + op.Object + " — queries dependentes podem regredir.",
			Migration: m.Path, Operation: string(op.Type),
			Object: op.Object,
		})
	case OpCreateIndex:
		out = append(out, Finding{
			Code:      "CREATE_INDEX",
			Level:     RiskLow,
			Message:   "CREATE INDEX " + op.Object + " — bloqueante em bases grandes; considere CONCURRENTLY (Postgres) ou ONLINE (MySQL 8).",
			Migration: m.Path, Operation: string(op.Type),
			Object: op.Object,
		})
	case OpUpdate:
		out = append(out, Finding{
			Code:      "UPDATE_DATA",
			Level:     RiskLow,
			Message:   "UPDATE em " + op.Object + " — confirme escopo (WHERE presente?).",
			Migration: m.Path, Operation: string(op.Type),
			Object: op.Object,
		})
	case OpDelete:
		out = append(out, Finding{
			Code:      "DELETE_DATA",
			Level:     RiskLow,
			Message:   "DELETE FROM " + op.Object + " — confirme escopo (WHERE presente?).",
			Migration: m.Path, Operation: string(op.Type),
			Object: op.Object,
		})
	}
	return out
}

// assessRaw faz análise textual do Raw statement para detectar padrões
// perigosos que a estrutura não pega (NOT NULL sem DEFAULT, CREATE INDEX
// sem CONCURRENTLY, UPDATE/DELETE sem WHERE).
func assessRaw(m Migration, op Operation) []Finding {
	var out []Finding
	raw := op.Raw
	if raw == "" {
		return out
	}
	up := strings.ToUpper(raw)

	switch op.Type {
	case OpCreateIndex:
		// Postgres: CREATE INDEX sem CONCURRENTLY é bloqueante.
		// MySQL: CREATE INDEX sem ONLINE é OK (MySQL 8 já é online por default).
		// Conservador: só marcamos se for detectado "CREATE INDEX" sem
		// "CONCURRENTLY" — confiamos no framework.
		if !strings.Contains(up, "CONCURRENTLY") && !strings.Contains(up, "ONLINE") {
			out = append(out, Finding{
				Code:      "CREATE_INDEX_BLOCKING",
				Level:     RiskModerate,
				Message:   "CREATE INDEX sem CONCURRENTLY (Postgres) — bloqueia escrita na tabela durante criação.",
				Migration: m.Path, Operation: string(op.Type),
				Object: op.Object,
			})
		}
	case OpUpdate:
		// UPDATE sem WHERE = potencial full-table update.
		if !regexp.MustCompile(`(?i)\bWHERE\b`).MatchString(raw) {
			out = append(out, Finding{
				Code:      "MASSIVE_UPDATE",
				Level:     RiskHigh,
				Message:   "UPDATE em " + op.Object + " sem WHERE — atualiza todas as linhas; transação longa.",
				Migration: m.Path, Operation: string(op.Type),
				Object: op.Object, Detail: "no WHERE clause",
			})
		}
	case OpDelete:
		if !regexp.MustCompile(`(?i)\bWHERE\b`).MatchString(raw) {
			out = append(out, Finding{
				Code:      "MASSIVE_DELETE",
				Level:     RiskCritical,
				Message:   "DELETE FROM " + op.Object + " sem WHERE — apaga TODAS as linhas.",
				Migration: m.Path, Operation: string(op.Type),
				Object: op.Object, Detail: "no WHERE clause",
			})
		}
	case OpAdd:
		// ADD COLUMN com NOT NULL sem DEFAULT = quebra em tabela com dados.
		if isAddColumnNotNullNoDefault(raw) {
			out = append(out, Finding{
				Code:      "NOT_NULL_NO_DEFAULT",
				Level:     RiskHigh,
				Message:   "ADD COLUMN com NOT NULL sem DEFAULT — falha em tabela com dados existentes; precisa backfill.",
				Migration: m.Path, Operation: string(op.Type),
				Object: op.Object, Detail: op.Detail,
			})
		}
	case OpAlter:
		// ALTER COLUMN ... SET NOT NULL (sem DEFAULT) em tabela com dados.
		if isAlterColumnNotNullNoDefault(raw) {
			out = append(out, Finding{
				Code:      "NOT_NULL_NO_DEFAULT",
				Level:     RiskHigh,
				Message:   "ALTER COLUMN SET NOT NULL sem DEFAULT prévio — falha em linhas existentes com NULL; precisa backfill.",
				Migration: m.Path, Operation: string(op.Type),
				Object: op.Object, Detail: op.Detail,
			})
		}
	}
	return out
}

// assessMetadata verifica metadata da migration: rollback ausente para
// frameworks que normalmente o suportam.
func assessMetadata(m Migration) []Finding {
	if m.HasRollback {
		return nil
	}
	if !frameworkSupportsRollback(m.Framework) {
		return nil
	}
	return []Finding{{
		Code:      "MISSING_ROLLBACK",
		Level:     RiskModerate,
		Message:   "Framework " + string(m.Framework) + " suporta rollback, mas nenhum vizinho down/rollback detectado.",
		Migration: m.Path, Object: m.Path, Detail: string(m.Framework),
	}}
}

// Impact é a taxonomia de 3 níveis pedida pelo contrato de dados §13
// (docs/dashboard-gap-mapping.md) — Baixo/Médio/Alto. Não é uma nova
// análise: é uma projeção do RiskLevel que Assess já calcula por
// finding, reaproveitando os mesmos sinais (sem reimplementar parsing).
// Mapeamento: LOW->Baixo, MODERATE->Médio, HIGH e CRITICAL colapsam em
// Alto (CRITICAL é um HIGH mais severo — DROP TABLE/SCHEMA — não uma 4ª
// categoria de impacto de negócio; ex.: "add column nullable" só gera
// finding ADD_COLUMN=LOW -> Baixo, "backfill" (NOT_NULL sem DEFAULT) gera
// HIGH -> Alto, DROP gera HIGH/CRITICAL -> Alto).
type Impact string

const (
	ImpactLow    Impact = "Baixo"
	ImpactMedium Impact = "Médio"
	ImpactHigh   Impact = "Alto"
)

var riskRank = map[RiskLevel]int{RiskLow: 0, RiskModerate: 1, RiskHigh: 2, RiskCritical: 3}

// ImpactForLevel projeta um RiskLevel isolado no enum de 3 níveis.
// RiskLevel desconhecido/vazio devolve "" — sem opinião, nunca chuta.
func ImpactForLevel(level RiskLevel) Impact {
	switch level {
	case RiskLow:
		return ImpactLow
	case RiskModerate:
		return ImpactMedium
	case RiskHigh, RiskCritical:
		return ImpactHigh
	default:
		return ""
	}
}

// ImpactForFindings agrega os findings de uma migration pelo pior
// (maior severidade) e projeta no enum de 3 níveis. Migration sem
// finding nenhum devolve "" — não fabrica "Baixo" como default só
// porque nada foi detectado; a ausência de sinal é ambígua, não é
// prova de baixo risco.
func ImpactForFindings(findings []Finding) Impact {
	if len(findings) == 0 {
		return ""
	}
	worst := RiskLow
	for _, f := range findings {
		if riskRank[f.Level] > riskRank[worst] {
			worst = f.Level
		}
	}
	return ImpactForLevel(worst)
}

// riskForDropTable define a severidade por tipo de objeto no DROP.
func riskForDropObject(kind string) RiskLevel {
	switch strings.ToUpper(kind) {
	case "TABLE":
		return RiskCritical
	case "VIEW", "TYPE", "FUNCTION", "TRIGGER", "SEQUENCE":
		return RiskHigh
	case "SCHEMA":
		return RiskCritical
	default:
		return RiskHigh
	}
}

// frameworkSupportsRollback lista os frameworks onde rollback é
// convenção (espera-se um arquivo down/.rollback).
func frameworkSupportsRollback(fw Framework) bool {
	switch fw {
	case Flyway, Liquibase, Rails, Django, EFCore, Laravel:
		return true
	}
	return false
}

// isAddColumnNotNullNoDefault devolve true se raw é um ADD COLUMN com
// NOT NULL e SEM DEFAULT na mesma linha.
func isAddColumnNotNullNoDefault(raw string) bool {
	up := strings.ToUpper(raw)
	if !strings.Contains(up, "ADD") {
		return false
	}
	// Procura padrão ADD COLUMN <col> <type> NOT NULL (sem DEFAULT antes do próximo comma/END).
	// Conservador: usa heurística simples.
	if !regexp.MustCompile(`(?i)ADD\s+(?:COLUMN\s+)?`).MatchString(raw) {
		return false
	}
	if !regexp.MustCompile(`(?i)\bNOT\s+NULL\b`).MatchString(raw) {
		return false
	}
	// Se tem DEFAULT, não é problema.
	if regexp.MustCompile(`(?i)\bDEFAULT\b`).MatchString(raw) {
		return false
	}
	return true
}

// isAlterColumnNotNullNoDefault devolve true se raw é ALTER COLUMN ... SET NOT NULL.
func isAlterColumnNotNullNoDefault(raw string) bool {
	up := strings.ToUpper(raw)
	if !regexp.MustCompile(`(?i)\bSET\s+NOT\s+NULL\b`).MatchString(up) {
		return false
	}
	// Se a mesma migration já fez ADD ... DEFAULT antes (no raw completo),
	// pode ser OK — mas se houver só SET NOT NULL sem DEFAULT prévio na
	// linha, é risco.
	return true
}
