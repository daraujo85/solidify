// Package migrations — parser SQL básico.
//
// Detecta CREATE/ALTER/DROP/ADD/RENAME/UPDATE/DELETE/CREATE_INDEX/
// DROP_INDEX em migrations SQL de modo CONSERVADOR. Se uma
// afirmação não bate um padrão conhecido, devolve unknown_operation
// em vez de inventar. Não tenta ser parser SQL universal —
// comentários, PL/SQL, funções e SQL dinâmico ficam UNKNOWN.
package migrations

import (
	"regexp"
	"strings"
)

// OperationType é o tipo canônico de operação de migration.
type OperationType string

const (
	OpCreate      OperationType = "CREATE"
	OpAlter       OperationType = "ALTER"
	OpDrop        OperationType = "DROP"
	OpAdd         OperationType = "ADD"
	OpRename      OperationType = "RENAME"
	OpUpdate      OperationType = "UPDATE"
	OpDelete      OperationType = "DELETE"
	OpCreateIndex OperationType = "CREATE_INDEX"
	OpDropIndex   OperationType = "DROP_INDEX"
	OpUnknown     OperationType = "UNKNOWN_OPERATION"
)

// Operation é uma operação detectada em uma migration SQL.
type Operation struct {
	Type   OperationType
	Object string // nome da tabela/view/index/type quando extraível
	Detail string // nome de coluna, novo nome, etc.
	Raw    string // statement original (trimado, sem comentários)
}

// Parse devolve as operações detectadas em sql. Conservador: cada
// statement que não bate um padrão vira uma entry OpUnknown (não
// inventa). Strings entre aspas simples e comentários são removidos
// antes do match para evitar armadilhas (comentários com "DROP" no
// meio, etc). Identificadores com aspas duplas (Postgres) são
// PRESERVADOS — precisamos do nome.
func Parse(sql string) []Operation {
	cleaned := stripNoise(sql)
	stmts := splitStatements(cleaned)
	out := make([]Operation, 0, len(stmts))
	for _, s := range stmts {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if op, ok := matchStatement(s); ok {
			out = append(out, op)
		} else {
			out = append(out, Operation{Type: OpUnknown, Raw: truncate(s, 200)})
		}
	}
	return out
}

// stripNoise remove comentários linha e bloco, e substitui conteúdo
// de strings literais ('...') por espaço — para que "DROP TABLE 'foo'"
// não seja confundido, etc. Identificadores com aspas duplas ("...")
// são PRESERVADOS: precisamos do nome do objeto.
func stripNoise(sql string) string {
	var b strings.Builder
	b.Grow(len(sql))
	i := 0
	for i < len(sql) {
		// Comentário linha: -- até \n.
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			continue
		}
		// Comentário bloco: /* ... */.
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			i += 2
			for i+1 < len(sql) && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			if i+1 < len(sql) {
				i += 2
			}
			continue
		}
		// String literal: '...' (escape '').
		if sql[i] == '\'' {
			b.WriteByte(' ')
			i++
			for i < len(sql) {
				if sql[i] == '\'' && i+1 < len(sql) && sql[i+1] == '\'' {
					i += 2 // escape ''
					continue
				}
				if sql[i] == '\'' {
					i++
					break
				}
				if sql[i] != '\n' {
					b.WriteByte(' ')
				}
				i++
			}
			continue
		}
		// Identifier com aspas duplas (Postgres): PRESERVA conteúdo.
		if sql[i] == '"' {
			b.WriteByte('"')
			i++
			for i < len(sql) && sql[i] != '"' {
				b.WriteByte(sql[i])
				i++
			}
			if i < len(sql) {
				b.WriteByte('"')
				i++
			}
			continue
		}
		b.WriteByte(sql[i])
		i++
	}
	return b.String()
}

// splitStatements divide em statements pelo ';'. Como stripNoise já
// removeu strings e comentários, splitting simples funciona.
func splitStatements(sql string) []string {
	var out []string
	for _, p := range strings.Split(sql, ";") {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// matchStatement devolve (op, true) se casa, senão (zero, false).
// IMPORTANTE: as regex aqui capturam UM ÚNICO grupo (o nome do
// objeto). Qualificadores opcionais como UNIQUE, IF NOT EXISTS, OR
// REPLACE NÃO viram grupos — eles bagunçam os índices quando não
// casam. Verificamos o statement inteiro para esses qualificadores.
func matchStatement(s string) (Operation, bool) {
	norm := whitespaceNormalize(strings.TrimSpace(s))
	up := strings.ToUpper(norm)

	// CREATE INDEX (incluindo UNIQUE, IF NOT EXISTS).
	if m := reCreateIndex.FindStringSubmatch(norm); m != nil {
		op := Operation{Type: OpCreateIndex, Object: stripQuotes(strings.ToLower(m[1])),
			Raw: truncate(s, 200)}
		if strings.Contains(up, "UNIQUE INDEX") || strings.Contains(up, "INDEX UNIQUE") {
			op.Detail = "UNIQUE"
		}
		return op, true
	}
	// DROP INDEX.
	if m := reDropIndex.FindStringSubmatch(norm); m != nil {
		return Operation{Type: OpDropIndex, Object: stripQuotes(strings.ToLower(m[1])),
			Raw: truncate(s, 200)}, true
	}
	// CREATE OR REPLACE — ambíguo entre CREATE e DROP.
	if strings.HasPrefix(up, "CREATE OR REPLACE") {
		if m := reCreate.FindStringSubmatch(norm); m != nil {
			return Operation{Type: OpUnknown, Object: stripQuotes(strings.ToLower(m[1])),
				Detail: "CREATE OR REPLACE", Raw: truncate(s, 200)}, true
		}
		return Operation{Type: OpUnknown, Detail: "CREATE OR REPLACE",
			Raw: truncate(s, 200)}, true
	}
	// CREATE TABLE/VIEW/TYPE/... (sem OR REPLACE).
	if m := reCreate.FindStringSubmatch(norm); m != nil {
		kind := extractKind(up, "TABLE|VIEW|TYPE|FUNCTION|TRIGGER|SEQUENCE|SCHEMA|EXTENSION")
		return Operation{Type: OpCreate, Object: stripQuotes(strings.ToLower(m[1])),
			Detail: kind, Raw: truncate(s, 200)}, true
	}
	// DROP TABLE/VIEW/TYPE/FUNCTION/... (genérico; DROP INDEX tratado acima).
	if m := reDropTable.FindStringSubmatch(norm); m != nil {
		return Operation{Type: OpDrop, Object: stripQuotes(strings.ToLower(m[2])),
			Detail: strings.ToUpper(m[1]), Raw: truncate(s, 200)}, true
	}
	// ALTER TABLE (com ou sem ONLY).
	if m := reAlter.FindStringSubmatch(norm); m != nil {
		table := stripQuotes(strings.ToLower(m[1]))
		// ADD CONSTRAINT é genérico (não é ADD COLUMN).
		if reAlterAddConstraint.MatchString(norm) {
			return Operation{Type: OpAlter, Object: table, Detail: "CONSTRAINT",
				Raw: truncate(s, 200)}, true
		}
		// ADD [COLUMN] <col>.
		if m2 := reAlterAddCol.FindStringSubmatch(norm); m2 != nil {
			return Operation{Type: OpAdd, Object: table,
				Detail: stripQuotes(strings.ToLower(m2[1])), Raw: truncate(s, 200)}, true
		}
		// DROP [COLUMN] <col>.
		if m2 := reAlterDropCol.FindStringSubmatch(norm); m2 != nil {
			return Operation{Type: OpDrop, Object: table,
				Detail: stripQuotes(strings.ToLower(m2[1])), Raw: truncate(s, 200)}, true
		}
		// RENAME TO <new>.
		if m2 := reAlterRenameTo.FindStringSubmatch(norm); m2 != nil {
			return Operation{Type: OpRename, Object: table,
				Detail: stripQuotes(strings.ToLower(m2[1])), Raw: truncate(s, 200)}, true
		}
		// RENAME [COLUMN] <old> TO <new>.
		if m2 := reAlterRenameCol.FindStringSubmatch(norm); m2 != nil {
			return Operation{Type: OpRename, Object: table,
				Detail: stripQuotes(strings.ToLower(m2[1])) + " -> " +
					stripQuotes(strings.ToLower(m2[2])),
				Raw: truncate(s, 200)}, true
		}
		// ALTER [COLUMN] <col>.
		if m2 := reAlterAlterCol.FindStringSubmatch(norm); m2 != nil {
			return Operation{Type: OpAlter, Object: table,
				Detail: stripQuotes(strings.ToLower(m2[1])), Raw: truncate(s, 200)}, true
		}
		// Outro ALTER (ADD CONSTRAINT, ALTER CONSTRAINT, etc).
		return Operation{Type: OpAlter, Object: table,
			Raw: truncate(s, 200)}, true
	}
	// UPDATE.
	if m := reUpdate.FindStringSubmatch(norm); m != nil {
		return Operation{Type: OpUpdate, Object: stripQuotes(strings.ToLower(m[1])),
			Raw: truncate(s, 200)}, true
	}
	// DELETE FROM.
	if m := reDelete.FindStringSubmatch(norm); m != nil {
		return Operation{Type: OpDelete, Object: stripQuotes(strings.ToLower(m[1])),
			Raw: truncate(s, 200)}, true
	}
	// INSERT INTO — UNKNOWN (não sabemos se é data op destrutiva).
	if regexp.MustCompile(`^INSERT\b`).MatchString(norm) {
		return Operation{Type: OpUnknown, Detail: "INSERT",
			Raw: truncate(s, 200)}, true
	}
	// BEGIN/COMMIT/ROLLBACK/SET/SELECT/WITH/TRUNCATE/etc — UNKNOWN.
	if regexp.MustCompile(`^(BEGIN|COMMIT|ROLLBACK|SET|SELECT|WITH|GRANT|REVOKE|TRUNCATE|CALL|EXPLAIN)\b`).MatchString(norm) {
		return Operation{Type: OpUnknown, Raw: truncate(s, 200)}, true
	}
	// PL/SQL ou function body — UNKNOWN explícito.
	trimmed := strings.TrimSpace(norm)
	if strings.HasPrefix(trimmed, "DECLARE") ||
		strings.HasPrefix(trimmed, "$$") ||
		strings.HasPrefix(trimmed, "BEGIN") ||
		strings.HasPrefix(trimmed, "END") {
		return Operation{Type: OpUnknown, Raw: truncate(s, 200)}, true
	}
	return Operation{}, false
}

// whitespaceNormalize colapsa múltiplos espaços/tabs/newlines.
func whitespaceNormalize(s string) string {
	return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
}

// extractKind devolve a primeira palavra que bate kindRe (em upper)
// presente em up. Sem captura, só match simples.
func extractKind(up, kindRe string) string {
	re := regexp.MustCompile(`(?i)\b(` + kindRe + `)\b`)
	if m := re.FindStringSubmatch(up); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}

// stripQuotes remove aspas duplas ao redor do identificador.
func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// Regexes compiladas. Cada regex captura UM ÚNICO grupo: o nome do
// objeto. Qualificadores opcionais NÃO viram grupos — bagunçam os
// índices quando não casam.
var (
	reCreateIndex = regexp.MustCompile(`(?i)^CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\S+)`)
	reDropIndex   = regexp.MustCompile(`(?i)^DROP\s+INDEX\s+(?:IF\s+EXISTS\s+)?(\S+)`)
	// CREATE [qualifiers] KIND [IF NOT EXISTS] name — grupo 1 = name.
	reCreate = regexp.MustCompile(`(?i)^CREATE\s+(?:(?:UNIQUE|TEMP|TEMPORARY|VIRTUAL|MATERIALIZED|UNLOGGED)\s+)?(?:TABLE|VIEW|TYPE|FUNCTION|TRIGGER|SEQUENCE|SCHEMA|EXTENSION)\s+(?:IF\s+NOT\s+EXISTS\s+)?(\S+)`)
	// DROP KIND [IF EXISTS] name — grupos: 1=kind, 2=name.
	reDropTable = regexp.MustCompile(`(?i)^DROP\s+(TABLE|VIEW|TYPE|FUNCTION|TRIGGER|SEQUENCE|SCHEMA|EXTENSION)\s+(?:IF\s+EXISTS\s+)?(\S+)`)
	// ALTER TABLE [ONLY] name — grupo 1 = table.
	reAlter = regexp.MustCompile(`(?i)^ALTER\s+TABLE\s+(?:ONLY\s+)?(\S+)`)
	// Detectado PRIMEIRO para diferenciar ADD CONSTRAINT de ADD COLUMN.
	reAlterAddConstraint = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(?:ONLY\s+)?\S+\s+ADD\s+CONSTRAINT\b`)
	reAlterAddCol        = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(?:ONLY\s+)?\S+\s+ADD\s+(?:COLUMN\s+)?(\S+)`)
	reAlterDropCol       = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(?:ONLY\s+)?\S+\s+DROP\s+(?:COLUMN\s+)?(\S+)`)
	reAlterRenameTo      = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(?:ONLY\s+)?\S+\s+RENAME\s+TO\s+(\S+)`)
	reAlterRenameCol     = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(?:ONLY\s+)?\S+\s+RENAME\s+(?:COLUMN\s+)?(\S+)\s+TO\s+(\S+)`)
	reAlterAlterCol      = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(?:ONLY\s+)?\S+\s+ALTER\s+(?:COLUMN\s+)?(\S+)`)
	reUpdate             = regexp.MustCompile(`(?i)^UPDATE\s+(?:ONLY\s+)?(\S+)`)
	reDelete             = regexp.MustCompile(`(?i)^DELETE\s+FROM\s+(\S+)`)
)

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
