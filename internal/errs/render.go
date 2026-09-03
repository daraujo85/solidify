package errs

import (
	"encoding/json"
	"errors"
	"io"
)

// RenderHuman escreve o erro em formato legível, uma linha por informação.
// Formato estável (golden test): não altere sem atualizar testdata.
func RenderHuman(w io.Writer, err error) error {
	if err == nil {
		return nil
	}

	var typed *Error
	if !errors.As(err, &typed) {
		typed = Wrap(CodeInternal, err.Error(), nil)
	}

	if _, werr := io.WriteString(w, "erro ["+string(typed.Code)+"]: "+typed.Msg+"\n"); werr != nil {
		return werr
	}
	if typed.Field != "" {
		if _, werr := io.WriteString(w, "  campo: "+typed.Field+"\n"); werr != nil {
			return werr
		}
	}
	if cause := unwrapMessage(typed); cause != "" {
		if _, werr := io.WriteString(w, "  causa: "+cause+"\n"); werr != nil {
			return werr
		}
	}
	if typed.Hint != "" {
		if _, werr := io.WriteString(w, "  ação: "+typed.Hint+"\n"); werr != nil {
			return werr
		}
	}
	return nil
}

// jsonError é a forma serializada de um erro. Campos vazios são omitidos para
// manter o payload estável e pequeno.
type jsonError struct {
	Code     Code   `json:"code"`
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
	Cause    string `json:"cause,omitempty"`
	Hint     string `json:"hint,omitempty"`
	ExitCode int    `json:"exit_code"`
}

// RenderJSON escreve o erro como um objeto JSON sob a chave "error".
func RenderJSON(w io.Writer, err error) error {
	if err == nil {
		return nil
	}

	var typed *Error
	if !errors.As(err, &typed) {
		typed = Wrap(CodeInternal, err.Error(), nil)
	}

	payload := struct {
		Error jsonError `json:"error"`
	}{jsonError{
		Code:     typed.Code,
		Message:  typed.Msg,
		Field:    typed.Field,
		Cause:    unwrapMessage(typed),
		Hint:     typed.Hint,
		ExitCode: typed.ExitCode(),
	}}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func unwrapMessage(e *Error) string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}
