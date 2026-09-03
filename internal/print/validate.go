// PDF contractual content validation (SAI-092).
//
// Valida que PDF gerado contém seções obrigatórias
// do release report (gate, scores, SOLID, risk).
// Extrai texto via heurística simples — sem lib PDF
// (mantém zero-deps).
package print

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

// RequiredSection marcador textual que deve aparecer.
type RequiredSection struct {
	ID      string
	Marker  string
	MinHits int
}

// DefaultRequiredSections seções mínimas p/ release
// contractual.
var DefaultRequiredSections = []RequiredSection{
	{ID: "gate", Marker: "Quality gate", MinHits: 1},
	{ID: "score", Marker: "Score", MinHits: 1},
	{ID: "solid", Marker: "SOLID", MinHits: 1},
	{ID: "risk", Marker: "Risk", MinHits: 1},
	{ID: "run_id", Marker: "run", MinHits: 1},
}

// ValidateResult saída.
type ValidateResult struct {
	Path           string         `json:"path"`
	Found          map[string]int `json:"found"`
	Missing        []string       `json:"missing,omitempty"`
	OK             bool           `json:"ok"`
	BytesExtracted int            `json:"bytes_extracted"`
}

// ExtractPDFText extrai blocos de texto do PDF bruto.
// Heurística: PDF armazena texto em streams
// descompactados como `(...)Tj` ou `<...>Tj`.
// Procuramos runs de bytes imprimíveis entre `(` e
// `)` ou `<` e `>`.
func ExtractPDFText(path string) (string, error) {
	if path == "" {
		return "", errors.New("path vazio")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(string(raw[:minBytes(4, len(raw))]), "%PDF") {
		return "", errors.New("não é PDF")
	}
	// SAI-129: Chrome subsetiza fontes e codifica Tj como IDs de glifo;
	// só o CMap /ToUnicode reconstrói texto. pdftotext (Poppler) já lida
	// com CMap, Flate, fontes Type3 e formatos PDF reais. Mantemos abaixo
	// o scanner zero-deps como fallback para PDF simples/fixtures e hosts
	// sem Poppler.
	if text, err := extractWithPDFToText(path); err == nil {
		return text, nil
	}
	// SAI-129 (achado validação e2e): content stream do Chrome
	// (--print-to-pdf) vem FlateDecode-comprimido — o scan de
	// "(...)Tj" abaixo nunca via texto real nos bytes crus, só
	// binário. Descomprime cada "stream...endstream" (zlib — é o
	// que /FlateDecode usa) antes de escanear; stream que não
	// descomprime (imagem/fonte/já-texto) entra como está, sem
	// custo extra pro caso comum.
	data := decompressStreams(raw)
	var sb strings.Builder
	in := false
	flush := func(start int, i int) {
		if i > start {
			chunk := string(data[start:i])
			if printable(chunk) {
				sb.WriteString(chunk)
				sb.WriteString("\n")
			}
		}
	}
	start := 0
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch c {
		case '(':
			flush(start, i)
			start = i + 1
			in = true
		case ')':
			if in {
				flush(start, i)
				start = i + 1
				in = false
			}
		case '<':
			if !in {
				flush(start, i)
				start = i + 1
				// hex stream — pula até '>'.
				for j := i + 1; j < len(data); j++ {
					if data[j] == '>' {
						start = j + 1
						i = j
						break
					}
				}
			}
		}
	}
	flush(start, len(data))
	return sb.String(), nil
}

var runPDFToText = func(path string) ([]byte, error) {
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		return nil, err
	}
	return exec.Command(bin, path, "-").Output()
}

func extractWithPDFToText(path string) (string, error) {
	out, err := runPDFToText(path)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

var streamMarker = []byte("stream")
var endstreamMarker = []byte("endstream")

// decompressStreams extrai cada bloco "stream...endstream" do PDF
// e tenta zlib-inflate; sucesso entra descomprimido no buffer,
// falha entra cru (mesma heurística de antes, sem regressão).
func decompressStreams(raw []byte) []byte {
	var out bytes.Buffer
	i := 0
	for {
		si := bytes.Index(raw[i:], streamMarker)
		if si < 0 {
			out.Write(raw[i:])
			break
		}
		si += i
		bodyStart := si + len(streamMarker)
		// "stream" é seguido por CRLF ou LF antes do conteúdo real.
		if bodyStart < len(raw) && raw[bodyStart] == '\r' {
			bodyStart++
		}
		if bodyStart < len(raw) && raw[bodyStart] == '\n' {
			bodyStart++
		}
		ei := bytes.Index(raw[bodyStart:], endstreamMarker)
		if ei < 0 {
			out.Write(raw[i:])
			break
		}
		ei += bodyStart
		body := raw[bodyStart:ei]
		if zr, err := zlib.NewReader(bytes.NewReader(body)); err == nil {
			if dec, err := io.ReadAll(zr); err == nil {
				out.Write(dec)
			} else {
				out.Write(body)
			}
		} else {
			out.Write(body)
		}
		out.WriteByte('\n')
		i = ei + len(endstreamMarker)
	}
	return out.Bytes()
}

func printable(s string) bool {
	if len(s) == 0 {
		return false
	}
	good := 0
	for _, r := range s {
		if r >= 32 && r < 127 || r == '\n' || r == '\t' {
			good++
		}
	}
	return good*4 >= len(s)*3 // 75%.
}

// ValidatePDF checa sections obrigatórias.
func ValidatePDF(path string, required []RequiredSection) (*ValidateResult, error) {
	text, err := ExtractPDFText(path)
	if err != nil {
		return nil, err
	}
	res := &ValidateResult{
		Path:           path,
		Found:          map[string]int{},
		BytesExtracted: len(text),
	}
	lower := strings.ToLower(text)
	for _, s := range required {
		count := strings.Count(lower, strings.ToLower(s.Marker))
		res.Found[s.ID] = count
		if count < s.MinHits {
			res.Missing = append(res.Missing, s.ID)
		}
	}
	res.OK = len(res.Missing) == 0
	return res, nil
}

func minBytes(a, b int) int {
	if a < b {
		return a
	}
	return b
}
