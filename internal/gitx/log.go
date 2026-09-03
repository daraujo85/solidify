package gitx

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/diegoaraujo/solidify/internal/errs"
)

// Commit é um commit retornado por `git log` com metadados parseados.
type Commit struct {
	SHA            string
	AuthorName     string
	AuthorEmail    string
	AuthorDate     string // ISO 8601
	CommitterName  string
	CommitterEmail string
	CommitterDate  string
	Subject        string // primeira linha do body
	Body           string // mensagem completa
	Conventional   Conventional
	IssueKeys      []string
}

// Conventional é a estrutura de uma mensagem Conventional Commit.
type Conventional struct {
	Type        string // feat, fix, refactor, perf, security, chore, …
	Scope       string // opcional; vazio se não houver
	Description string // depois de "type(scope)?!: "
	Breaking    bool   // '!' após type/scope, ou "BREAKING CHANGE" no footer
}

// LogOpts ajusta o `git log`.
type LogOpts struct {
	// MaxCommits limita a quantidade devolvida. 0 = sem limite.
	MaxCommits int
	// IssueKeyPatterns são regexes aplicados a Subject+Body. Matches viram
	// IssueKeys. nil = não extrai (default).
	IssueKeyPatterns []*regexp.Regexp
}

// Log devolve os commits em rng, em ordem do mais novo pro mais antigo.
//
// Implementação: 1 chamada a `git rev-list` pra listar SHAs + N chamadas a
// `git log -1` por SHA. O(N) é aceitável pra ranges de release (tipicamente
// < 100 commits) e elimina fragilidade do `git log --format` com separadores
// longos (o git 2.47 absorve o separador final antes do newline).
func Log(dir, rng string, opts LogOpts) ([]Commit, error) {
	if strings.TrimSpace(rng) == "" {
		return nil, errs.New(errs.CodeUsage, "range é obrigatório").
			WithField("git.range")
	}

	shas, err := revList(dir, rng, opts.MaxCommits)
	if err != nil {
		return nil, err
	}

	commits := make([]Commit, 0, len(shas))
	for _, sha := range shas {
		c, err := showCommit(dir, sha, opts)
		if err != nil {
			return nil, err
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// revList devolve os SHAs em rng. Ordem newest-first.
//
// git rev-list A..B exclui A; para o caller (que usa os SHAs pra metadados)
// a lista precisa incluir A. Então prepend o A do range, depois aplica o
// limite de MaxCommits sobre o total.
func revList(dir, rng string, max int) ([]string, error) {
	args := []string{"--no-pager", "rev-list", "--no-color"}
	if max > 0 {
		args = append(args, "-n", strconv.Itoa(max))
	}
	args = append(args, rng)
	out, err := runGitCmd(dir, args...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	shas := strings.Split(strings.TrimSpace(out), "\n")
	// Se rng é "A..B", prepend A para incluir o commit mais antigo.
	if parts := strings.SplitN(rng, "..", 2); len(parts) == 2 && parts[0] != "" {
		first := parts[0]
		if len(shas) == 0 || shas[len(shas)-1] != first {
			shas = append(shas, first)
		}
	}
	if max > 0 && len(shas) > max {
		shas = shas[:max]
	}
	return shas, nil
}

// showCommit devolve os metadados de um SHA usando `--format` com separador
// interno (ASCII Unit Separator, 0x1F). Git nunca absorve separadores
// *entre* campos; só o último separador antes do newline é descartado.
func showCommit(dir, sha string, opts LogOpts) (Commit, error) {
	// 7 separadores entre 8 campos; sem trailing porque git encerra com \n.
	args := []string{"--no-pager", "log", "--no-color", "-1",
		"--format=%H%x1f%an%x1f%ae%x1f%aI%x1f%cn%x1f%ce%x1f%cI%x1f%B",
		sha,
	}
	out, err := runGitCmd(dir, args...)
	if err != nil {
		return Commit{}, err
	}
	fields := strings.Split(out, "\x1f")
	if len(fields) < 8 {
		return Commit{}, errs.Newf(errs.CodeGit,
			"saída inesperada do git log %s: %d campos", sha, len(fields))
	}
	c := Commit{
		SHA:            strings.TrimSpace(fields[0]),
		AuthorName:     fields[1],
		AuthorEmail:    fields[2],
		AuthorDate:     fields[3],
		CommitterName:  fields[4],
		CommitterEmail: fields[5],
		CommitterDate:  fields[6],
		Body:           strings.TrimRight(fields[7], "\n"),
	}
	body := c.Body
	if idx := strings.IndexByte(body, '\n'); idx >= 0 {
		c.Subject = body[:idx]
	} else {
		c.Subject = body
	}
	c.Conventional = parseConventional(c.Subject, c.Body)
	if len(opts.IssueKeyPatterns) > 0 {
		c.IssueKeys = matchIssueKeys(c.Subject+"\n"+c.Body, opts.IssueKeyPatterns)
	}
	return c, nil
}

// conventionalPattern captura type, scope (opcional) e breaking opcional.
var conventionalPattern = regexp.MustCompile(
	`^(?P<type>[a-z]+)(?:\((?P<scope>[^)]+)\))?(?P<breaking>!)?: (?P<description>.+)$`,
)

// breakingFooterPattern detecta footer "BREAKING CHANGE:" ou "BREAKING-CHANGE:".
var breakingFooterPattern = regexp.MustCompile(
	`(?m)^BREAKING[ -]CHANGE:`,
)

// parseConventional extrai type/scope/breaking de um commit.
//
// Regras:
//   - subject bate Conventional (regex acima) → type, scope, breaking, description;
//   - "BREAKING CHANGE"/"BREAKING-CHANGE" no body ou footer também marca breaking;
//   - qualquer outro subject vira Conventional.Type="other" e resto em branco.
func parseConventional(subject, body string) Conventional {
	m := conventionalPattern.FindStringSubmatch(subject)
	if m == nil {
		return Conventional{Type: "other", Description: subject}
	}
	c := Conventional{
		Type:        m[1],
		Scope:       m[2],
		Breaking:    m[3] == "!",
		Description: m[4],
	}
	if !c.Breaking && breakingFooterPattern.MatchString(body) {
		c.Breaking = true
	}
	return c
}

// matchIssueKeys devolve os matches únicos do texto contra os regexes.
func matchIssueKeys(text string, patterns []*regexp.Regexp) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, p := range patterns {
		for _, m := range p.FindAllString(text, -1) {
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			out = append(out, m)
		}
	}
	return out
}
