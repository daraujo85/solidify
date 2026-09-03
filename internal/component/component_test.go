package component

import (
	"sort"
	"testing"

	"github.com/diegoaraujo/solidify/internal/manifests"
	"github.com/diegoaraujo/solidify/internal/stack"
)

func m(path string, kind manifests.Kind) manifests.Manifest {
	return manifests.Manifest{Path: path, Kind: kind, Dir: dirOf(path)}
}

// dirOf devolve o diretório do path do manifest.
func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}

func det(s stack.Stack, conf float64) stack.Detection {
	return stack.Detection{Stack: s, Confidence: conf, Evidence: nil}
}

func names(d []Distribution) []string {
	out := make([]string, 0, len(d))
	for _, x := range d {
		if len(x.Paths) > 0 {
			out = append(out, string(x.Component)+":"+itoa(len(x.Paths)))
		}
	}
	sort.Strings(out)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// Aceitação: cada prefixo reconhece seu componente.
func TestClassifyByPrefix(t *testing.T) {
	cases := []struct {
		path string
		want Component
	}{
		{"apps/web/src/App.tsx", FrontendWeb},
		{"apps/api/handlers/user.go", BackendAPI},
		{"services/worker/jobs.go", Worker},
		{"packages/ui/src/Button.tsx", FrontendWeb}, // "ui" casa antes de "packages" — frontend-web vence por prioridade
		{"packages/shared/types.ts", Library},
		{"mobile/ios/View.swift", Mobile},
		{"infra/terraform/main.tf", Infra},
		{"migrations/001_init.sql", Database},
		{"db/schema/users.sql", Database},
		{"prisma/schema.prisma", Database},
		{"alembic/versions/001.py", Database},
		{"lib/utils.js", Library},
		{"libs/shared/types.ts", Library},
		{"jobs/email/sender.go", Worker},
		{"server/routes/auth.go", BackendAPI},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := Classify([]string{tc.path}, nil, nil); got != tc.want {
				t.Errorf("%s → %s, quero %s", tc.path, got, tc.want)
			}
		})
	}
}

// Aceitação: database vence sobre outros prefixos no mesmo range.
func TestClassifyDatabaseWinsOverAPI(t *testing.T) {
	got := Classify([]string{
		"apps/api/handlers/user.go",
		"migrations/2024_add_email.sql",
	}, nil, nil)
	if got != Database {
		t.Errorf("database + api → %s, quero Database", got)
	}
}

// Aceitação: infra vence sobre backend no mesmo range.
func TestClassifyInfraWinsOverBackend(t *testing.T) {
	got := Classify([]string{
		"apps/api/handlers/user.go",
		"infra/terraform/main.tf",
	}, nil, nil)
	if got != Infra {
		t.Errorf("infra + api → %s, quero Infra", got)
	}
}

// Aceitação: monorepo frontend + backend — ClassifyAll particiona.
func TestClassifyAllMonorepo(t *testing.T) {
	paths := []string{
		"apps/web/src/App.tsx",
		"apps/web/src/Header.tsx",
		"apps/api/handlers/user.go",
		"apps/api/handlers/order.go",
		"packages/ui/src/Button.tsx",
	}
	d := ClassifyAll(paths, nil, nil)
	got := names(d)
	want := []string{"backend-api:2", "frontend-web:3"} // packages/ui → frontend-web (prioridade "ui")
	if !equalStr(got, want) {
		t.Errorf("partição = %v, quero %v", got, want)
	}
}

func equalStr(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Aceitação: commit backend-only → BackendAPI.
func TestClassifyBackendOnlyCommit(t *testing.T) {
	got := Classify([]string{
		"apps/api/server.go",
		"apps/api/handlers/health.go",
	}, nil, nil)
	if got != BackendAPI {
		t.Errorf("backend-only → %s, quero BackendAPI", got)
	}
}

// Aceitação: Flutter detectado + path dentro do escopo → Mobile.
func TestClassifyFlutterStackForcesMobile(t *testing.T) {
	ms := []manifests.Manifest{
		m("apps/mobile/pubspec.yaml", manifests.KindFlutterPubspec),
	}
	ds := []stack.Detection{det(stack.StackFlutter, 1.0)}
	got := Classify([]string{
		"apps/mobile/lib/main.dart",
		"apps/mobile/lib/widget.dart",
	}, ms, ds)
	if got != Mobile {
		t.Errorf("Flutter scope → %s, quero Mobile", got)
	}
}

// Aceitação: Flutter stack mas path fora do escopo → não vira mobile.
func TestClassifyFlutterStackOutOfScope(t *testing.T) {
	ms := []manifests.Manifest{
		m("apps/mobile/pubspec.yaml", manifests.KindFlutterPubspec),
	}
	ds := []stack.Detection{det(stack.StackFlutter, 1.0)}
	got := Classify([]string{
		"apps/web/src/App.tsx", // fora do mobile
	}, ms, ds)
	if got != FrontendWeb {
		t.Errorf("Flutter scope + web path → %s, quero FrontendWeb", got)
	}
}

// Aceitação: paths vazios + sem stack → Unknown.
func TestClassifyEmpty(t *testing.T) {
	if got := Classify(nil, nil, nil); got != Unknown {
		t.Errorf("empty → %s, quero Unknown", got)
	}
	if got := Classify([]string{}, nil, nil); got != Unknown {
		t.Errorf("[]string{} → %s, quero Unknown", got)
	}
}

// Aceitação: paths sem nenhum prefixo conhecido → Unknown.
func TestClassifyUnknownPaths(t *testing.T) {
	got := Classify([]string{
		"foo/bar/baz.go",
		"misc/x.txt",
	}, nil, nil)
	if got != Unknown {
		t.Errorf("paths aleatórios → %s, quero Unknown", got)
	}
}

// Aceitação: paths múltiplos — maioria backend → BackendAPI.
func TestClassifyMajorityWins(t *testing.T) {
	got := Classify([]string{
		"apps/api/a.go",
		"apps/api/b.go",
		"apps/api/c.go",
		"apps/web/d.tsx",
	}, nil, nil)
	if got != BackendAPI {
		t.Errorf("3 backend + 1 frontend → %s, quero BackendAPI", got)
	}
}

// Aceitação: paths frontend-web em profundidade (/web/) também casam.
func TestClassifyFrontendContains(t *testing.T) {
	got := Classify([]string{"src/web/App.tsx"}, nil, nil)
	if got != FrontendWeb {
		t.Errorf("src/web/App.tsx → %s, quero FrontendWeb", got)
	}
}

// Aceitação: manifestos em diretórios contam no score.
func TestClassifyManifestsContributeToScore(t *testing.T) {
	// Sem paths, só manifests em "api/" → BackendAPI.
	ms := []manifests.Manifest{
		m("apps/api/go.mod", manifests.KindGoMod),
	}
	got := Classify(nil, ms, nil)
	if got != BackendAPI {
		t.Errorf("manifests em apps/api → %s, quero BackendAPI", got)
	}
}
