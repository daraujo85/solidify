// Package gitx resolve refs Git para SHAs e computa o range de análise.
//
// O Solidify depende de git só para descobrir o diff e o histórico: shelling out
// para o binário `git` evita reinventar o protocolo de objetos. O binário
// precisa estar disponível (PATH); sua ausência é erro de configuração.
//
// Os métodos não tocam em estado fora do repositório alvo: cada comando git
// é executado com Dir=D e sem环境污染 entre execuções.
package gitx

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/diegoaraujo/solidify/internal/errs"
)

// Resolver configura a resolução do range. Reutilizável e stateless depois de
// construído.
type Resolver struct {
	// Dir é o repositório onde rodar os comandos git. Vazio = usar o cwd.
	Dir string
	// UseMergeBase, quando true, computa merge-base(base, head) e usa esse SHA
	// como início efetivo do range. Quando false, range é base..head direto.
	UseMergeBase bool
	// SingleCommit, quando true, interpreta head como commit único: o range
	// efetivo é head^..head. Só faz sentido quando head é um SHA (ou ref que
	// aponta para um commit isolado).
	SingleCommit bool
}

// Scope é o resultado da resolução.
type Scope struct {
	// Base e Head são as refs originais pedidas (antes da resolução).
	Base string
	Head string
	// DiffStrategy identifica a semântica usada para calcular o delivery diff.
	DiffStrategy string
	// BaseSHA e HeadSHA são os SHAs completos (40 hex) após `git rev-parse`.
	BaseSHA string
	HeadSHA string
	// MergeBaseSHA é o SHA do merge-base, preenchido quando UseMergeBase=true.
	// Vazio caso contrário.
	MergeBaseSHA string
	// Range é a expressão de revisão pronta para `git log`/`git diff`:
	// "mergeBase..head" ou "base..head" (ou "head^..head" em SingleCommit).
	Range string
}

// Resolve valida e resolve as refs para SHAs.
//
// Comportamento:
//   - base="" ou head="" são erro (uso);
//   - refs são resolvidas com `git rev-parse --verify` (falha se não existem);
//   - objetos são validados com `git cat-file -e` (proteção contra refs
//     quebradas que rev-parse aceita mas o repo não materializa);
//   - SingleCommit: base deve ser "" e head é tratado como commit único;
//     o range vira head^..head; se head não tem pai (commit raiz), é erro.
func (r *Resolver) Resolve(base, head string) (Scope, error) {
	if strings.TrimSpace(head) == "" {
		return Scope{}, errs.New(errs.CodeUsage, "head é obrigatório").
			WithField("git.head")
	}

	if r.SingleCommit {
		if strings.TrimSpace(base) != "" {
			return Scope{}, errs.New(errs.CodeUsage,
				"single commit não aceita base explícita").
				WithField("git.base")
		}
		return r.resolveSingle(head)
	}

	if strings.TrimSpace(base) == "" {
		return Scope{}, errs.New(errs.CodeUsage, "base é obrigatório").
			WithField("git.base")
	}

	baseSHA, err := r.revParse(base)
	if err != nil {
		return Scope{}, errs.Newf(errs.CodeGit, "base inválida: %s", base).
			WithField("git.base").WithHint("verifique a ref: branch, tag, SHA ou remote tracking")
	}
	headSHA, err := r.revParse(head)
	if err != nil {
		return Scope{}, errs.Newf(errs.CodeGit, "head inválida: %s", head).
			WithField("git.head")
	}
	if err := r.catFileOK(baseSHA); err != nil {
		return Scope{}, errs.Newf(errs.CodeGit, "objeto base ausente no repo: %s", baseSHA)
	}
	if err := r.catFileOK(headSHA); err != nil {
		return Scope{}, errs.Newf(errs.CodeGit, "objeto head ausente no repo: %s", headSHA)
	}

	out := Scope{
		Base:    base,
		Head:    head,
		BaseSHA: baseSHA,
		HeadSHA: headSHA,
		DiffStrategy: "endpoints",
	}

	if r.UseMergeBase {
		mb, err := r.mergeBase(baseSHA, headSHA)
		if err != nil {
			return Scope{}, err
		}
		if err := r.catFileOK(mb); err != nil {
			return Scope{}, errs.Newf(errs.CodeGit,
				"objeto merge-base ausente no repo: %s", mb)
		}
		out.MergeBaseSHA = mb
		out.DiffStrategy = "merge-base"
		out.Range = mb + ".." + headSHA
	} else {
		out.Range = baseSHA + ".." + headSHA
	}
	return out, nil
}

func (r *Resolver) resolveSingle(head string) (Scope, error) {
	headSHA, err := r.revParse(head)
	if err != nil {
		return Scope{}, errs.Newf(errs.CodeGit, "head inválida: %s", head).
			WithField("git.head")
	}
	if err := r.catFileOK(headSHA); err != nil {
		return Scope{}, errs.Newf(errs.CodeGit, "objeto head ausente no repo: %s", headSHA)
	}
	parentSHA, err := r.revParse(head + "^")
	if err != nil {
		return Scope{}, errs.Newf(errs.CodeGit,
			"head não tem parent (commit raiz); single commit exige histórico").
			WithField("git.head")
	}
	if err := r.catFileOK(parentSHA); err != nil {
		return Scope{}, errs.Newf(errs.CodeGit, "objeto parent ausente no repo: %s", parentSHA)
	}
	return Scope{
		Base:    head + "^",
		Head:    head,
		BaseSHA: parentSHA,
		HeadSHA: headSHA,
		Range:   parentSHA + ".." + headSHA,
	}, nil
}

// revParse devolve o SHA canônico (40 hex) de uma ref.
func (r *Resolver) revParse(ref string) (string, error) {
	out, err := r.runGit("rev-parse", "--verify", ref+"^{commit}")
	return out, err
}

// catFileOK valida que o objeto existe no repo local.
func (r *Resolver) catFileOK(sha string) error {
	_, err := r.runGit("cat-file", "-e", sha)
	if err != nil {
		return errs.Newf(errs.CodeGit, "objeto git ausente: %s", sha)
	}
	return nil
}

// mergeBase devolve o SHA do ancestral comum mais próximo entre base e head.
func (r *Resolver) mergeBase(baseSHA, headSHA string) (string, error) {
	out, err := r.runGit("merge-base", baseSHA, headSHA)
	if err != nil {
		return "", errs.Newf(errs.CodeGit,
			"merge-base(%s, %s) falhou: branches sem ancestral comum?", baseSHA, headSHA)
	}
	return out, nil
}

// runGit executa git com args a partir de r.Dir. Devolve stdout trimado.
// Erros de execução são embrulhados com errs.CodeGit e mensagem do stderr.
func (r *Resolver) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errs.Newf(errs.CodeGit, "git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// EnsureBinary falha cedo se `git` não estiver no PATH. Rodar uma vez no
// startup da CLI para que mensagens de erro sejam acionáveis, não "not found".
func EnsureBinary() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return errs.New(errs.CodeGit,
			"binário `git` não encontrado no PATH").
			WithHint("instale git ou ajuste PATH para incluir o diretório do executável")
	}
	return nil
}
