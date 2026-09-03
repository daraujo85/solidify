package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestJSONGoldenShape(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{Out: &buf, Level: slog.LevelInfo, Format: FormatJSON, OmitTime: true})
	logger.Info("analisando range", "base", "abc123", "head", "def456", "files", 12)

	got := strings.TrimSpace(buf.String())
	want := `{"level":"INFO","msg":"analisando range","base":"abc123","head":"def456","files":12}`
	if got != want {
		t.Errorf("linha JSON = %s\nquero      = %s", got, want)
	}
}

func TestTextGoldenShape(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{Out: &buf, Level: slog.LevelWarn, Format: FormatText, OmitTime: true})
	logger.Warn("lighthouse ignorado", "motivo", "backend-only")

	got := strings.TrimSpace(buf.String())
	want := `level=WARN msg="lighthouse ignorado" motivo=backend-only`
	if got != want {
		t.Errorf("linha texto = %s\nquero       = %s", got, want)
	}
}

// Aceite crítico: um valor de secret jamais chega ao writer.
func TestSecretAttrsAreRedacted(t *testing.T) {
	const secret = "abc123super"
	sensitiveKeys := []string{
		"token", "PAYMENT_TOKEN", "api_key", "apiKey", "password",
		"authorization", "sonar_secret", "db_connection_string",
		"session_id", "cookie", "env_value", "private_key", "key",
	}

	for _, key := range sensitiveKeys {
		t.Run(key, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New(Options{Out: &buf, Level: slog.LevelInfo, Format: FormatJSON, OmitTime: true})
			logger.Info("evento", key, secret)

			if strings.Contains(buf.String(), secret) {
				t.Fatalf("secret vazou para o log: %s", buf.String())
			}
			if !strings.Contains(buf.String(), Redacted) {
				t.Fatalf("valor não foi marcado como redigido: %s", buf.String())
			}
		})
	}
}

func TestNonSensitiveKeysArePreserved(t *testing.T) {
	for _, key := range []string{"cache_key", "public_key_path", "component", "files", "base"} {
		var buf bytes.Buffer
		logger := New(Options{Out: &buf, Level: slog.LevelInfo, Format: FormatJSON, OmitTime: true})
		logger.Info("evento", key, "valor-visivel")

		if key == "public_key_path" {
			// contém "private_key"? não; contém "key" como substring, mas não é
			// igual a "key" — deve permanecer visível.
			if !strings.Contains(buf.String(), "valor-visivel") {
				t.Errorf("chave %q foi mascarada indevidamente: %s", key, buf.String())
			}
			continue
		}
		if !strings.Contains(buf.String(), "valor-visivel") {
			t.Errorf("chave %q foi mascarada indevidamente: %s", key, buf.String())
		}
	}
}

func TestLevelFilteringDropsBelowThreshold(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{Out: &buf, Level: slog.LevelWarn, Format: FormatJSON, OmitTime: true})
	logger.Debug("invisível")
	logger.Info("invisível")
	logger.Warn("visível")

	if strings.Contains(buf.String(), "invisível") {
		t.Errorf("nível abaixo do threshold foi emitido: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "visível") {
		t.Errorf("nível acima do threshold foi suprimido: %s", buf.String())
	}
}

func TestOmitTimeRemovesTimestamp(t *testing.T) {
	var buf bytes.Buffer
	New(Options{Out: &buf, Format: FormatJSON, OmitTime: true}).Info("x")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if _, ok := line["time"]; ok {
		t.Errorf("timestamp presente com OmitTime: %s", buf.String())
	}

	buf.Reset()
	New(Options{Out: &buf, Format: FormatJSON}).Info("x")
	if !strings.Contains(buf.String(), `"time"`) {
		t.Errorf("timestamp ausente sem OmitTime: %s", buf.String())
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]struct {
		want slog.Level
		ok   bool
	}{
		"debug":     {slog.LevelDebug, true},
		"INFO":      {slog.LevelInfo, true},
		"":          {slog.LevelInfo, true},
		"warn":      {slog.LevelWarn, true},
		" warning ": {slog.LevelWarn, true},
		"error":     {slog.LevelError, true},
		"trace":     {slog.LevelInfo, false},
	}
	for input, want := range cases {
		got, ok := ParseLevel(input)
		if got != want.want || ok != want.ok {
			t.Errorf("ParseLevel(%q) = %v,%v; quero %v,%v", input, got, ok, want.want, want.ok)
		}
	}
}

func TestParseFormat(t *testing.T) {
	if got, ok := ParseFormat("JSON"); got != FormatJSON || !ok {
		t.Errorf("ParseFormat(JSON) = %v,%v", got, ok)
	}
	if got, ok := ParseFormat("xml"); got != FormatText || ok {
		t.Errorf("ParseFormat(xml) = %v,%v; quero text,false", got, ok)
	}
}

func TestDiscardWritesNothing(t *testing.T) {
	// Não deve entrar em pânico nem escrever; apenas exercita o caminho.
	Discard().Error("nada acontece")
}
