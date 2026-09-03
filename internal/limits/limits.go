// Tool resource limits (SAI-099).
//
// Define profiles compose-style com limites de CPU/
// memória/concurrency por tool. Heavy tools rodam com
// concurrency=1 (evita OOM e races).
package limits

// Profile preset.
type Profile struct {
	Name        string
	CPU         string  // ex: "0.5", "1.0", "2.0"
	Memory      string  // ex: "512m", "1g", "2g"
	Concurrency int     // workers paralelos
	Heavy       bool    // se true, força concurrency=1
	Notes       string
}

// StandardProfiles presets.
var StandardProfiles = map[string]Profile{
	"light": {
		Name:        "light",
		CPU:         "0.25",
		Memory:      "256m",
		Concurrency: 4,
		Notes:       "linters, formatadores, static analysis",
	},
	"medium": {
		Name:        "medium",
		CPU:         "0.5",
		Memory:      "512m",
		Concurrency: 2,
		Notes:       "testes unitários, cobertura",
	},
	"heavy": {
		Name:        "heavy",
		CPU:         "2.0",
		Memory:      "2g",
		Concurrency: 1, // forçado
		Heavy:       true,
		Notes:       "k6, lighthouse, e2e, load tests",
	},
}

// Limits limites por tool.
type Limits struct {
	Tool    string
	Profile string // chave em StandardProfiles
}

// DefaultLimits limites default por tool.
var DefaultLimits = map[string]Limits{
	"k6":         {"k6", "heavy"},
	"lighthouse": {"lighthouse", "heavy"},
	"playwright": {"playwright", "heavy"},
	"jest":       {"jest", "medium"},
	"go-test":    {"go-test", "medium"},
	"eslint":     {"eslint", "light"},
	"prettier":   {"prettier", "light"},
}

// EffectiveConcurrency retorna concurrency efetivo
// (força 1 se Heavy).
func EffectiveConcurrency(p Profile) int {
	if p.Heavy {
		return 1
	}
	if p.Concurrency < 1 {
		return 1
	}
	return p.Concurrency
}

// Resolve retorna profile expandido p/ um tool.
func Resolve(tool string) (Profile, bool) {
	l, ok := DefaultLimits[tool]
	if !ok {
		return Profile{}, false
	}
	p, ok := StandardProfiles[l.Profile]
	if !ok {
		return Profile{}, false
	}
	// override do nome com tool.
	p.Name = tool
	return p, true
}

// ComposeFragment gera fragmento compose-style p/ tool.
//
// Formato: `cpus: X, mem_limit: Y, concurrency: Z`.
// Caller formata em YAML/compose real.
func ComposeFragment(p Profile) string {
	out := "cpus: \"" + p.CPU + "\""
	out += "\nmem_limit: \"" + p.Memory + "\""
	out += "\nconcurrency: " + itoaL(p.Concurrency)
	if p.Heavy {
		out += "\n# heavy: concurrency forçado em 1"
	}
	return out
}

func itoaL(n int) string {
	if n == 0 {
		return "0"
	}
	out := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	if neg {
		out = append([]byte{'-'}, out...)
	}
	return string(out)
}

// ValidateProfile checa consistência.
func ValidateProfile(p Profile) error {
	if p.CPU == "" {
		return errorsL("limits: CPU vazio")
	}
	if p.Memory == "" {
		return errorsL("limits: Memory vazio")
	}
	if p.Heavy && p.Concurrency > 1 {
		return errorsL("limits: heavy com concurrency > 1")
	}
	return nil
}

type stringErr string

func (s stringErr) Error() string { return string(s) }

func errorsL(s string) error { return stringErr(s) }
