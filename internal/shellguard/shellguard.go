// Shell guard (SAI-102).
//
// Valida inputs antes de passar pra shell. Detecta
// padrões de command injection em paths e commit
// subjects.
//
// Estratégia: blacklist de chars perigosos + validação
// de formato. Caller DEVE usar `exec.Command` com args
// separados, nunca `sh -c "..."`.
package shellguard

import (
	"errors"
	"strings"
)

// ErrInjection padrão de injeção detectado.
var ErrInjection = errors.New("shellguard: injection detectado")

// ErrEmptyArg arg vazio.
var ErrEmptyArg = errors.New("shellguard: arg vazio")

// DangerousChars chars que nunca devem aparecer em
// args que viram shell args.
//
// NOTA: `(` e `)` removidos — conventional commits usam
// `feat(scope):` e não devem bloquear.
var DangerousChars = []string{
	";", "&", "|", "`", "$", "<", ">",
	"\n", "\r", "\\", "'", "\"",
}

// ShellMetachars metachars de shell.
var ShellMetachars = []string{
	"&&", "||", ">>", "<<", "$()", "${", "};", "|;",
}

// ValidateArg checa se arg é seguro p/ shell.
func ValidateArg(arg string) error {
	if arg == "" {
		return ErrEmptyArg
	}
	for _, c := range DangerousChars {
		if strings.Contains(arg, c) {
			return ErrInjection
		}
	}
	for _, m := range ShellMetachars {
		if strings.Contains(arg, m) {
			return ErrInjection
		}
	}
	return nil
}

// ValidateArgs checa múltiplos args.
func ValidateArgs(args []string) error {
	for i, a := range args {
		if err := ValidateArg(a); err != nil {
			return err
		}
		_ = i
	}
	return nil
}

// SanitizeSubject valida commit subject (mensagem curta,
// sem chars de controle). Aceita `()` por convenção.
func SanitizeSubject(subject string) error {
	if subject == "" {
		return ErrEmptyArg
	}
	if len(subject) > 200 {
		return errors.New("shellguard: subject > 200 chars")
	}
	for _, c := range DangerousChars {
		if strings.Contains(subject, c) {
			return ErrInjection
		}
	}
	for _, m := range ShellMetachars {
		if strings.Contains(subject, m) {
			return ErrInjection
		}
	}
	return nil
}

// SanitizePath valida path (sem shell chars + sem
// traversal).
func SanitizePath(p string) error {
	if p == "" {
		return ErrEmptyArg
	}
	if strings.Contains(p, "..") {
		return ErrInjection
	}
	for _, c := range DangerousChars {
		if strings.Contains(p, c) {
			return ErrInjection
		}
	}
	return nil
}

// HasMetachar retorna true se arg tem metachar.
func HasMetachar(arg string) bool {
	for _, c := range DangerousChars {
		if strings.Contains(arg, c) {
			return true
		}
	}
	for _, m := range ShellMetachars {
		if strings.Contains(arg, m) {
			return true
		}
	}
	return false
}
