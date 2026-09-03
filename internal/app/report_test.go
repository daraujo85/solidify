package app

import (
	"bytes"
	"strings"
	"testing"
	"io"
	"log/slog"

	"github.com/diegoaraujo/solidify/internal/errs"
)

func TestRunReport_FormatUnknown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := Env{Stdout: &stdout, Stderr: &stderr}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := runReport([]string{"--format=unknown"}, env, logger)
	if err == nil {
		t.Fatal("esperava erro, veio nil")
	}

	code := errs.CodeOf(err)
	if code != errs.CodeUsage {
		t.Errorf("esperava CodeUsage, veio %v", code)
	}
	if !strings.Contains(err.Error(), "formato desconhecido: unknown") {
		t.Errorf("erro inesperado: %v", err)
	}
}

func TestRunReport_PDF_NoRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := Env{Stdout: &stdout, Stderr: &stderr}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := runReport([]string{"--format=pdf"}, env, logger)
	if err == nil {
		t.Fatal("esperava erro, veio nil")
	}

	code := errs.CodeOf(err)
	if code != errs.CodeUsage {
		t.Errorf("esperava CodeUsage, veio %v", code)
	}
	if !strings.Contains(err.Error(), "--run é obrigatório para --format=pdf") {
		t.Errorf("erro inesperado: %v", err)
	}
}
