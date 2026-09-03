// Package logging configura o logger estruturado do Solidify.
//
// Invariantes:
//   - todo log vai para stderr; stdout é reservado para saída de dados;
//   - nenhum valor de env/secret é logado — atributos com chave secret-like
//     são mascarados antes de chegar ao handler (guardrail mínimo; o
//     framework completo de redaction é SAI-020);
//   - o formato JSON é opt-in e estável o suficiente para golden test.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// Format é o formato de saída do logger.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Redacted substitui qualquer valor considerado sensível.
const Redacted = "[REDACTED]"

// Options configura o logger.
type Options struct {
	// Out recebe os logs. Zero value usa os.Stderr no chamador.
	Out io.Writer
	// Level mínimo emitido.
	Level slog.Level
	// Format text (default) ou json.
	Format Format
	// OmitTime remove o timestamp. Usado em testes golden.
	OmitTime bool
	// OmitSource remove file:line mesmo em debug.
	OmitSource bool
}

// ParseLevel converte um nome de nível em slog.Level.
func ParseLevel(name string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug, true
	case "info", "":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

// ParseFormat converte um nome de formato em Format.
func ParseFormat(name string) (Format, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "text", "":
		return FormatText, true
	case "json":
		return FormatJSON, true
	default:
		return FormatText, false
	}
}

// New constrói o logger. Source só é anexado em debug, para não pagar o custo
// de stack lookup no caminho normal.
func New(opts Options) *slog.Logger {
	addSource := opts.Level <= slog.LevelDebug && !opts.OmitSource

	handlerOpts := &slog.HandlerOptions{
		Level:       opts.Level,
		AddSource:   addSource,
		ReplaceAttr: replaceAttr(opts.OmitTime),
	}

	var handler slog.Handler
	if opts.Format == FormatJSON {
		handler = slog.NewJSONHandler(opts.Out, handlerOpts)
	} else {
		handler = slog.NewTextHandler(opts.Out, handlerOpts)
	}
	return slog.New(handler)
}

func replaceAttr(omitTime bool) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if omitTime && len(groups) == 0 && a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		if IsSensitiveKey(a.Key) {
			return slog.String(a.Key, Redacted)
		}
		return a
	}
}

// sensitiveFragments são fragmentos de nome de atributo que indicam valor
// sensível. Comparação case-insensitive por substring: prefere falso positivo
// (mascarar demais) a vazamento.
var sensitiveFragments = []string{
	"secret",
	"token",
	"password",
	"passwd",
	"credential",
	"authorization",
	"auth_header",
	"apikey",
	"api_key",
	"private_key",
	"session",
	"cookie",
	"env_value",
	"dsn",
	"connection_string",
}

// IsSensitiveKey informa se um nome de atributo indica conteúdo sensível.
func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range sensitiveFragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	// "key" isolado é sensível; "public_key"/"cache_key" não. Trata só o exato
	// para evitar mascarar metadados legítimos como "cache_key".
	return lower == "key"
}

// Discard devolve um logger que não escreve nada. Útil em testes.
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}
