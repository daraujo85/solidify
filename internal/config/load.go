package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/diegoaraujo/solidify/internal/errs"
)

// FileName é o nome canônico do arquivo de config.
const FileName = "solidify.json"

// Loaded é o resultado de um carregamento: config efetiva + procedência.
type Loaded struct {
	Config Config
	// Path do arquivo lido; vazio quando só os defaults foram usados.
	Path string
	// FromFile indica se um arquivo foi encontrado.
	FromFile bool
	// Hash é o hash da config efetiva, após overrides.
	Hash string
}

// Load lê o solidify.json em dir (ou o próprio arquivo, se path apontar para um),
// aplica overrides, valida e calcula o hash.
//
// Ausência de arquivo não é erro: os defaults são uma config válida.
func Load(path string, overrides Overrides) (Loaded, error) {
	resolved, exists, err := resolvePath(path)
	if err != nil {
		return Loaded{}, err
	}

	cfg := Default()
	if exists {
		data, rerr := os.ReadFile(resolved)
		if rerr != nil {
			return Loaded{}, errs.Wrap(errs.CodeIO, "falha ao ler "+resolved, rerr)
		}
		if derr := decodeStrict(data, &cfg, resolved); derr != nil {
			return Loaded{}, derr
		}
	}

	if aerr := overrides.apply(&cfg); aerr != nil {
		return Loaded{}, aerr
	}
	if verr := cfg.Validate(); verr != nil {
		return Loaded{}, verr
	}

	hash, herr := cfg.Hash()
	if herr != nil {
		return Loaded{}, herr
	}

	loaded := Loaded{Config: cfg, FromFile: exists, Hash: hash}
	if exists {
		loaded.Path = resolved
	}
	return loaded, nil
}

// resolvePath aceita "" (cwd), um diretório ou o caminho do arquivo.
//
// Semântica de "não encontrado": procurar em um diretório e não achar é normal
// (os defaults valem). Apontar para um *arquivo* que não existe é erro — o
// usuário pediu aquele arquivo especificamente.
func resolvePath(path string) (string, bool, error) {
	candidate := path
	fromDir := false
	if candidate == "" {
		candidate = "."
	}

	info, err := os.Stat(candidate)
	switch {
	case err == nil && info.IsDir():
		candidate = filepath.Join(candidate, FileName)
		fromDir = true
	case err != nil && path != "":
		if errors.Is(err, os.ErrNotExist) {
			return "", false, errs.New(errs.CodeNotFound, "config não encontrada: "+path).
				WithHint("rode `solidify init` para criar " + FileName)
		}
		return "", false, errs.Wrap(errs.CodeIO, "falha ao inspecionar "+path, err)
	}

	if _, statErr := os.Stat(candidate); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			if fromDir || path == "" {
				return "", false, nil
			}
			return "", false, errs.New(errs.CodeNotFound, "config não encontrada: "+candidate).
				WithHint("rode `solidify init` para criar " + FileName)
		}
		return "", false, errs.Wrap(errs.CodeIO, "falha ao inspecionar "+candidate, statErr)
	}
	return candidate, true, nil
}

// decodeStrict rejeita campo desconhecido (equivalente ao
// additionalProperties:false do schema) e traduz erros de JSON em erros
// tipados com arquivo, linha e campo.
func decodeStrict(data []byte, cfg *Config, path string) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	if err := dec.Decode(cfg); err != nil {
		return describeDecodeError(err, data, path)
	}
	// Nada além de um único objeto JSON é aceito.
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return errs.New(errs.CodeConfig, "conteúdo extra após o objeto JSON").WithField(path)
	}
	return nil
}

func describeDecodeError(err error, data []byte, path string) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = "(raiz)"
		}
		return errs.Newf(errs.CodeConfig, "tipo inválido: esperava %s, veio %s", typeErr.Type, typeErr.Value).
			WithField(fmt.Sprintf("%s:%s: %s", path, location(data, typeErr.Offset), field))
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return errs.Wrap(errs.CodeConfig, "JSON inválido", syntaxErr).
			WithField(fmt.Sprintf("%s:%s", path, location(data, syntaxErr.Offset))).
			WithHint("o Solidify aceita apenas JSON; YAML e TOML não são suportados")
	}

	// DisallowUnknownFields não expõe tipo próprio; a mensagem é estável.
	if name, ok := unknownFieldName(err); ok {
		return errs.Newf(errs.CodeConfig, "campo desconhecido: %s", name).
			WithField(path).
			WithHint("remova o campo ou confira schemas/solidify-config.schema.json")
	}
	return errs.Wrap(errs.CodeConfig, "falha ao decodificar "+path, err)
}

func unknownFieldName(err error) (string, bool) {
	const prefix = "json: unknown field "
	msg := err.Error()
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return "", false
	}
	return strings.Trim(msg[idx+len(prefix):], `"`), true
}

// location traduz um offset de bytes em "linha:coluna", 1-based.
func location(data []byte, offset int64) string {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	line, col := 1, 1
	for _, b := range data[:offset] {
		if b == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return fmt.Sprintf("%d:%d", line, col)
}

// Hash devolve o SHA-256 da config efetiva.
//
// Determinismo: json.Marshal emite campos de struct na ordem de declaração e
// chaves de map ordenadas, portanto a serialização é estável entre execuções.
// O hash entra na chave de cache dos analyzers (SAI-029): mudar config
// invalida cache.
func (c Config) Hash() (string, error) {
	// O campo $schema é decoração de editor e não afeta comportamento.
	c.Schema = ""

	data, err := json.Marshal(c)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternal, "falha ao serializar config para hash", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// ResolveProfile resolve a cadeia `extends` e devolve o perfil efetivo com
// todos os campos preenchidos.
func (c Config) ResolveProfile(name string) (ResolvedProfile, error) {
	chain, err := c.profileChain(name)
	if err != nil {
		return ResolvedProfile{}, err
	}

	out := ResolvedProfile{Name: name}
	// Do ancestral mais distante para o mais próximo: o mais específico ganha.
	for i := len(chain) - 1; i >= 0; i-- {
		p := c.Profiles[chain[i]]
		setBool(&out.RequirePeerA, p.RequirePeerA)
		setBool(&out.RequirePeerB, p.RequirePeerB)
		setBool(&out.RequireArbiter, p.RequireArbiter)
		setBool(&out.RequireDistinctExternalModels, p.RequireDistinctExternalModels)
		setBool(&out.GeneratePDF, p.GeneratePDF)
		setBool(&out.ActiveSecurity, p.ActiveSecurity)
		setBool(&out.FailOnRequiredAnalyzerSkip, p.FailOnRequiredAnalyzerSkip)
		setBool(&out.RunRoleSwap, p.RunRoleSwap)
	}
	return out, nil
}

// profileChain devolve [nome, pai, avô, ...], detectando ciclos.
func (c Config) profileChain(name string) ([]string, error) {
	var chain []string
	seen := map[string]bool{}

	for current := name; current != ""; {
		profile, ok := c.Profiles[current]
		if !ok {
			return nil, errs.Newf(errs.CodeConfig, "perfil desconhecido: %s", current).
				WithField("profiles").
				WithHint("perfis disponíveis: " + strings.Join(c.profileNames(), ", "))
		}
		if seen[current] {
			return nil, errs.Newf(errs.CodeConfig, "ciclo de extends em profiles: %s", strings.Join(append(chain, current), " -> ")).
				WithField("profiles." + current + ".extends")
		}
		seen[current] = true
		chain = append(chain, current)
		current = profile.Extends
	}
	return chain, nil
}

// ResolvedProfile é um perfil com a cadeia extends já aplicada.
type ResolvedProfile struct {
	Name                          string
	RequirePeerA                  bool
	RequirePeerB                  bool
	RequireArbiter                bool
	RequireDistinctExternalModels bool
	GeneratePDF                   bool
	ActiveSecurity                bool
	FailOnRequiredAnalyzerSkip    bool
	RunRoleSwap                   bool
}

func setBool(dst *bool, src *bool) {
	if src != nil {
		*dst = *src
	}
}

func (c Config) profileNames() []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

// sortStrings evita importar sort só para isto; a lista é minúscula.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
