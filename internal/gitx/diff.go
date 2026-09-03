package gitx

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/diegoaraujo/solidify/internal/errs"
)

// Hunk é um trecho contíguo de mudança dentro de um arquivo.
type Hunk struct {
	FilePath           string
	OldStart, OldLines int
	NewStart, NewLines int
	// Header é a linha "@@ -X,Y +A,B @@ contexto" como veio do git.
	Header string
	// Content são as linhas do diff com prefixos (' ', '-', '+') preservados.
	Content string
	// Hash é um ID estável: muda se o diff muda, não muda se só as linhas
	// de contexto mudarem (cache por hash sobrevive a tweaks de context).
	Hash string
}

// DiffFile é o diff de um arquivo.
type DiffFile struct {
	Path    string
	OldPath string
	Status  ChangeKind
	Hunks   []Hunk
}

// DiffOpts ajusta o diff.
type DiffOpts struct {
	// ContextLines controla as linhas de contexto em volta de cada mudança.
	// 0 ou negativo = usar o default (3). O config (git.diff_context_lines)
	// sobrescreve este valor via DiffWithConfig.
	ContextLines int
	// MaxHunkBytes limita o conteúdo de cada hunk (em bytes). 0 = sem limite.
	// Hunk truncado ainda é parseado inteiro — o limite só afeta o campo
	// Content. Hash é computado sobre o conteúdo completo para que o cache
	// não colida entre runs com limites diferentes.
	MaxHunkBytes int
}

// Diff executa `git diff -U{N}` para rng e devolve hunks por arquivo.
//
// Streaming: usa bufio.Scanner sobre a saída do git. Cada linha é processada
// uma vez; nada é reconstruído em memória após o parse.
func Diff(dir, rng string, opts DiffOpts) ([]DiffFile, error) {
	if strings.TrimSpace(rng) == "" {
		return nil, errs.New(errs.CodeUsage, "range é obrigatório").
			WithField("git.range")
	}
	context := opts.ContextLines
	if context <= 0 {
		context = 3
	}

	args := []string{
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--patch",
		"--unified=" + strconv.Itoa(context),
		rng,
	}
	cmd := gitCmd(dir, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errs.Wrap(errs.CodeIO, "abrir pipe do git diff", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, errs.Wrap(errs.CodeGit, "iniciar git diff", err)
	}

	scanner := bufio.NewScanner(stdout)
	// Hunks podem ter linhas longas (especialmente com paths); aumenta o buffer.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var files []DiffFile
	var current *DiffFile
	var currentHunk *hunkBuilder
	var pendingHunkContent strings.Builder
	var inHunk bool

	flushHunk := func() {
		if currentHunk == nil || current == nil {
			return
		}
		currentHunk.content = pendingHunkContent.String()
		current.Hunks = append(current.Hunks, currentHunk.finalize(current.Path))
		currentHunk = nil
		pendingHunkContent.Reset()
		inHunk = false
	}

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushHunk()
			files = append(files, DiffFile{})
			current = &files[len(files)-1]
			current.Path = extractDiffPath(line)
			current.OldPath = current.Path
			// Default: se nenhum dos cabeçalhos abaixo marcar, é modificação.
			current.Status = ChangeModified
			currentHunk = nil
			inHunk = false
		case strings.HasPrefix(line, "rename from "):
			if current != nil {
				current.OldPath = strings.TrimPrefix(line, "rename from ")
				current.Status = ChangeRenamed
			}
		case strings.HasPrefix(line, "copy from "):
			if current != nil {
				current.OldPath = strings.TrimPrefix(line, "copy from ")
				current.Status = ChangeCopied
			}
		case strings.HasPrefix(line, "new file mode"):
			if current != nil {
				current.Status = ChangeAdded
			}
		case strings.HasPrefix(line, "deleted file mode"):
			if current != nil {
				current.Status = ChangeDeleted
			}
		case strings.HasPrefix(line, "old mode ") || strings.HasPrefix(line, "new mode "):
			// mode change indicator (T em --raw); não muda status principal.
		case strings.HasPrefix(line, "Binary files "):
			flushHunk()
			if current != nil {
				current.Status = ChangeModified
			}
			// Sem hunks para binário; segue para o próximo arquivo.
		case strings.HasPrefix(line, "@@"):
			flushHunk()
			oldStart, oldLines, newStart, newLines, _, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			currentHunk = &hunkBuilder{
				filePath: current.Path,
				oldStart: oldStart,
				oldLines: oldLines,
				newStart: newStart,
				newLines: newLines,
				header:   line,
			}
			inHunk = true
		case inHunk && currentHunk != nil:
			// Linha dentro do hunk: começa com ' ', '-', '+', ou '\\' (newline).
			if len(line) > 0 {
				first := line[0]
				if first != ' ' && first != '-' && first != '+' && first != '\\' {
					// Não é linha de hunk — provavelmente metadata de fim.
					flushHunk()
					continue
				}
			}
			if opts.MaxHunkBytes == 0 || pendingHunkContent.Len() < opts.MaxHunkBytes {
				pendingHunkContent.WriteString(line)
				pendingHunkContent.WriteByte('\n')
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errs.Wrap(errs.CodeIO, "ler saída do git diff", err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, errs.Wrap(errs.CodeGit, "git diff falhou", err)
	}
	flushHunk()
	return files, nil
}

// hunkBuilder é o estado mutável enquanto o hunk é parseado.
type hunkBuilder struct {
	filePath           string
	oldStart, oldLines int
	newStart, newLines int
	header             string
	content            string
}

func (h *hunkBuilder) finalize(path string) Hunk {
	return Hunk{
		FilePath: h.filePath,
		OldStart: h.oldStart,
		OldLines: h.oldLines,
		NewStart: h.newStart,
		NewLines: h.newLines,
		Header:   h.header,
		Content:  strings.TrimRight(h.content, "\n"),
		Hash:     computeHunkHash(path, h),
	}
}

// computeHunkHash devolve o ID estável: path + primeira linha alterada
// (relativa ao oldStart) + conteúdo core. Contexto NÃO entra no hash, então
// mudar ContextLines não invalida o cache.
//
// A coordenada da primeira mudança é a que sobrevive a tweaks de contexto:
// é sempre a mesma posição absoluta no arquivo. Já o oldStart do cabeçalho
// varia porque o git o aponta para a primeira linha do bloco (incluindo
// contexto).
func computeHunkHash(path string, h *hunkBuilder) string {
	var core strings.Builder
	var firstOldLine int
	oldCursor := h.oldStart
	for _, line := range strings.Split(strings.TrimRight(h.content, "\n"), "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case ' ':
			oldCursor++
		case '-':
			if firstOldLine == 0 {
				firstOldLine = oldCursor
			}
			core.WriteString(line)
			core.WriteByte('\n')
		case '+':
			core.WriteString(line)
			core.WriteByte('\n')
		}
	}
	if firstOldLine == 0 {
		// Hunk só com context (raro mas válido em git): ancora no oldStart.
		firstOldLine = h.oldStart
	}
	seed := path + "@" + strconv.Itoa(firstOldLine) + "|" + core.String()
	sum := sha256.Sum256([]byte(seed))
	return "hunk:" + hex.EncodeToString(sum[:])
}

// extractDiffPath extrai o path do "diff --git a/foo b/foo". Para paths com
// espaços, git faz quoting com aspas ou escapando — pegamos a forma entre
// " b/" e o fim.
func extractDiffPath(line string) string {
	const sep = " b/"
	idx := strings.LastIndex(line, sep)
	if idx < 0 {
		return ""
	}
	p := line[idx+len(sep):]
	// Remove aspas escapadas que git adiciona em paths com espaços.
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		inner := p[1 : len(p)-1]
		// Decodifica escapes comuns: \" e \\.
		var b strings.Builder
		for i := 0; i < len(inner); i++ {
			if inner[i] == '\\' && i+1 < len(inner) {
				b.WriteByte(inner[i+1])
				i++
				continue
			}
			b.WriteByte(inner[i])
		}
		return b.String()
	}
	return p
}

// parseHunkHeader parseia "@@ -10,5 +20,7 @@ opcional contexto".
func parseHunkHeader(line string) (oldStart, oldLines, newStart, newLines int, headerCtx string, ok bool) {
	if !strings.HasPrefix(line, "@@") {
		return 0, 0, 0, 0, "", false
	}
	// Encontra o segundo "@@".
	first := strings.Index(line, "@@")
	rest := line[first+2:]
	second := strings.Index(rest, "@@")
	if second < 0 {
		return 0, 0, 0, 0, "", false
	}
	middle := rest[:second]
	headerCtx = strings.TrimSpace(rest[second+2:])

	parts := strings.Fields(middle)
	if len(parts) < 2 {
		return 0, 0, 0, 0, "", false
	}
	os, oerr := parseRangePart(parts[0])
	ns, nerr := parseRangePart(parts[1])
	if oerr != nil || nerr != nil {
		return 0, 0, 0, 0, "", false
	}
	return os.start, os.count, ns.start, ns.count, headerCtx, true
}

type rangePart struct {
	start, count int
}

func parseRangePart(s string) (rangePart, error) {
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimPrefix(s, "+")
	comma := strings.IndexByte(s, ',')
	if comma < 0 {
		// Forma curta: "@@ -5 +5 @@" significa count=1.
		n, err := strconv.Atoi(s)
		if err != nil {
			return rangePart{}, err
		}
		return rangePart{start: n, count: 1}, nil
	}
	start, err := strconv.Atoi(s[:comma])
	if err != nil {
		return rangePart{}, err
	}
	count, err := strconv.Atoi(s[comma+1:])
	if err != nil {
		return rangePart{}, err
	}
	return rangePart{start: start, count: count}, nil
}

// BlobAt lê o conteúdo de path na ref (ex: "HEAD", "abc123", "main").
// Usa `git show <ref>:<path>` — equivalente a `git cat-file -p`.
func BlobAt(dir, ref, path string) (string, error) {
	if strings.TrimSpace(ref) == "" || strings.TrimSpace(path) == "" {
		return "", errs.New(errs.CodeUsage, "ref e path são obrigatórios").
			WithField("git.blob")
	}
	out, err := runGitCmd(dir, "show", ref+":"+path)
	if err != nil {
		return "", errs.Newf(errs.CodeGit, "blob %s:%s indisponível", ref, path)
	}
	return out, nil
}

func runGitCmd(dir string, args ...string) (string, error) {
	cmd := gitCmd(dir, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", errs.Newf(errs.CodeGit, "git %s: %s",
				strings.Join(args, " "), msg)
		}
		return "", errs.Newf(errs.CodeGit, "git %s: %v",
			strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}
