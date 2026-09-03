// Package errs define erros tipados com códigos estáveis para a CLI.
//
// Os códigos são parte do contrato observável do binário: scripts e agentes
// dependem deles. Nunca renomeie um código existente; adicione um novo.
package errs

import (
	"errors"
	"fmt"
)

// Code é o identificador estável de uma classe de falha.
type Code string

const (
	CodeOK       Code = "ok"
	CodeInternal Code = "internal"       // bug no Solidify
	CodeUsage    Code = "usage"          // uso incorreto da CLI
	CodeConfig   Code = "config_invalid" // solidify.json inválido
	CodeNotFound Code = "not_found"      // arquivo/ref/run inexistente
	CodeGit      Code = "git"            // repositório ou range inválido
	CodeIO       Code = "io"             // filesystem/artifact store
	CodeStorage  Code = "storage"        // SQLite
	CodeAnalyzer Code = "analyzer"       // ferramenta externa falhou
	CodeProvider   Code = "provider"   // provider de LLM inacessível/inválido
	CodeSchema     Code = "schema"     // payload fora do JSON Schema
	CodeSecurity   Code = "security"   // target não allowlisted, path traversal
	CodeTimeout    Code = "timeout"
	CodeCanceled   Code = "canceled"
	CodeIncomplete Code = "incomplete" // SAI-116: gate INCOMPLETE (schema/provider falhou; re-run)
	CodeGateFail    Code = "gate_fail"    // SAI-129: gate FAIL — run completo, score abaixo do threshold
	CodeGateBlocked Code = "gate_blocked" // SAI-129: gate BLOCKED — run completo, veredicto bloqueante não-score
)

// exitCodes mapeia Code para exit status do processo.
//
// Faixas: 1 interno, 2 uso, 3-9 entrada/ambiente, 20+ execução.
// A faixa 10-19 é reservada para veredictos de Quality Gate (SAI-075, SAI-116):
// 10 = gate FAIL, 11 = gate BLOCKED, 12 = gate INCOMPLETE.
var exitCodes = map[Code]int{
	CodeOK:         0,
	CodeInternal:   1,
	CodeUsage:      2,
	CodeConfig:     3,
	CodeNotFound:   4,
	CodeGit:        5,
	CodeIO:         6,
	CodeStorage:    7,
	CodeSecurity:   8,
	CodeSchema:     9,
	CodeGateFail:    10,
	CodeGateBlocked: 11,
	CodeIncomplete:  12,
	CodeAnalyzer:   20,
	CodeProvider:   21,
	CodeTimeout:    22,
	CodeCanceled:   23,
}

// Error é o erro tipado do Solidify.
type Error struct {
	Code Code
	// Msg é a mensagem para humanos. Nunca inclua valores de secret/env.
	Msg string
	// Hint é uma ação sugerida, opcional.
	Hint string
	// Field é o caminho do campo problemático (ex.: "ai.provider.base_url").
	Field string
	// Err é a causa encadeada, opcional.
	Err error
}

func (e *Error) Error() string {
	if e.Field != "" {
		return string(e.Code) + ": " + e.Field + ": " + e.Msg
	}
	return string(e.Code) + ": " + e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

// ExitCode devolve o exit status do processo para este erro.
func (e *Error) ExitCode() int { return e.Code.ExitCode() }

// ExitCode devolve o exit status associado ao código; desconhecido vira 1.
func (c Code) ExitCode() int {
	if code, ok := exitCodes[c]; ok {
		return code
	}
	return exitCodes[CodeInternal]
}

// New cria um erro tipado.
func New(code Code, msg string) *Error {
	return &Error{Code: code, Msg: msg}
}

// Newf cria um erro tipado com formatação.
//
// Cuidado: não interpole valores de env/secret no formato.
func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// Wrap encadeia uma causa sob um código tipado.
func Wrap(code Code, msg string, err error) *Error {
	return &Error{Code: code, Msg: msg, Err: err}
}

// WithHint devolve uma cópia com hint definido.
func (e *Error) WithHint(hint string) *Error {
	clone := *e
	clone.Hint = hint
	return &clone
}

// WithField devolve uma cópia com field definido.
func (e *Error) WithField(field string) *Error {
	clone := *e
	clone.Field = field
	return &clone
}

// CodeOf extrai o Code de um erro; erros não tipados são CodeInternal.
// nil devolve CodeOK.
func CodeOf(err error) Code {
	if err == nil {
		return CodeOK
	}
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return CodeInternal
}

// ExitCodeOf devolve o exit status para qualquer erro.
func ExitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	return CodeOf(err).ExitCode()
}
