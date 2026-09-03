package gitx

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/diegoaraujo/solidify/internal/errs"
)

// gitCmd monta um exec.Cmd pronto para rodar com Dir=dir.
func gitCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd
}

// ChangeKind é a categoria da mudança.
type ChangeKind int

const (
	ChangeUnknown ChangeKind = iota
	ChangeAdded
	ChangeModified
	ChangeDeleted
	ChangeRenamed
	ChangeCopied
	ChangeTypeChanged // mode-only change (sem diff de conteúdo)
)

func (k ChangeKind) String() string {
	switch k {
	case ChangeAdded:
		return "added"
	case ChangeModified:
		return "modified"
	case ChangeDeleted:
		return "deleted"
	case ChangeRenamed:
		return "renamed"
	case ChangeCopied:
		return "copied"
	case ChangeTypeChanged:
		return "type_changed"
	default:
		return "unknown"
	}
}

// FileChange é uma mudança observada em um range.
type FileChange struct {
	// Path é o caminho pós-mudança. Para rename/copy, é o destino.
	Path string
	// OldPath é o caminho pré-mudança (só preenchido em rename/copy).
	OldPath string
	// Status é a categoria da mudança.
	Status ChangeKind
	// Add e Del são as linhas adicionadas/removidas. -1 indica "não
	// contável" (binário, symlink, submodule).
	Add, Del int
	// IsBinary é true quando o arquivo não tem contagem de linhas.
	IsBinary bool
	// ModeChange indica que src e dst modes diferem.
	ModeChange bool
	// SourceMode / DestMode são os modos octal como string (ex.: "100644").
	SourceMode, DestMode string
}

// ChangesOpts ajusta a detecção.
type ChangesOpts struct {
	RenameDetection bool
	CopyDetection   bool
}

// Changes lista os arquivos alterados em rng.
//
// Usa `git diff --raw -z` para status/modos/paths e `git diff --numstat -z`
// para contagem de linhas (binário vira "-"). As duas saídas são casadas
// pelo path pós-mudança.
func Changes(dir, rng string, opts ChangesOpts) ([]FileChange, error) {
	if strings.TrimSpace(rng) == "" {
		return nil, errs.New(errs.CodeUsage, "range é obrigatório").
			WithField("git.range")
	}

	rawArgs := []string{"diff", "--raw", "-z"}
	rawArgs = appendRenameCopy(rawArgs, opts)
	rawArgs = append(rawArgs, rng)
	raw, err := gitDiffList(dir, rawArgs)
	if err != nil {
		return nil, err
	}

	statArgs := []string{"diff", "--numstat", "-z"}
	statArgs = appendRenameCopy(statArgs, opts)
	statArgs = append(statArgs, rng)
	stats, err := gitDiffNumstat(dir, statArgs)
	if err != nil {
		return nil, err
	}

	out := make([]FileChange, 0, len(raw))
	for _, r := range raw {
		fc := FileChange{
			Path:       r.path,
			OldPath:    r.oldPath,
			Status:     r.kind,
			SourceMode: r.srcMode,
			DestMode:   r.dstMode,
		}
		fc.ModeChange = r.srcMode != "" && r.dstMode != "" && r.srcMode != r.dstMode
		if s, ok := stats[r.path]; ok {
			fc.Add, fc.Del = s.add, s.del
			fc.IsBinary = s.binary
		}
		out = append(out, fc)
	}
	return out, nil
}

// appendRenameCopy anexa -M e -C de acordo com opts.
func appendRenameCopy(args []string, opts ChangesOpts) []string {
	if opts.RenameDetection {
		args = append(args, "-M")
	}
	if opts.CopyDetection {
		args = append(args, "-C")
	}
	return args
}

// rawEntry é o registro parseado de `git diff --raw -z`.
type rawEntry struct {
	kind             ChangeKind
	path, oldPath    string
	srcMode, dstMode string
}

// parseRawEntries decodifica o output de `git diff --raw -z`.
//
// Formato (tokenizado em NUL):
//
//	"<:src_mode dst_mode src_sha dst_sha status>" NUL "<path>" NUL
//	"<:src_mode dst_mode src_sha dst_sha status>" NUL "<old>" NUL "<new>" NUL  (rename/copy)
func parseRawEntries(data string) []rawEntry {
	var entries []rawEntry
	parts := strings.Split(data, "\x00")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p == "" || p[0] != ':' {
			continue
		}
		fields := strings.Fields(p[1:])
		if len(fields) < 5 {
			continue
		}
		kind, _ := parseStatus(fields[4])
		entry := rawEntry{
			kind:    kind,
			srcMode: fields[0],
			dstMode: fields[1],
		}
		i++
		if i >= len(parts) || parts[i] == "" {
			break
		}
		first := parts[i]
		if kind == ChangeRenamed || kind == ChangeCopied {
			i++
			if i >= len(parts) {
				break
			}
			entry.oldPath = first
			entry.path = parts[i]
		} else {
			entry.path = first
		}
		entries = append(entries, entry)
	}
	return entries
}

// parseStatus interpreta "M", "A", "R100", "C75", "T". Score só é
// informativo para rename/copy.
func parseStatus(raw string) (ChangeKind, int) {
	if raw == "" {
		return ChangeUnknown, 0
	}
	letter := raw[0]
	scoreStr := raw[1:]
	score := 0
	if scoreStr != "" {
		if v, err := strconv.Atoi(scoreStr); err == nil {
			score = v
		}
	}
	switch letter {
	case 'A':
		return ChangeAdded, score
	case 'M':
		return ChangeModified, score
	case 'D':
		return ChangeDeleted, score
	case 'R':
		return ChangeRenamed, score
	case 'C':
		return ChangeCopied, score
	case 'T':
		return ChangeTypeChanged, score
	default:
		return ChangeUnknown, score
	}
}

// statEntry é o registro de numstat por path (sempre o destino em rename/copy).
type statEntry struct {
	add, del int
	binary   bool
}

// parseNumstatEntries decodifica `git diff --numstat -z`.
//
// Formato (tokenizado em NUL):
//
//	"<add>\t<del>\t<path>" NUL                       (added/modified/deleted)
//	"<add>\t<del>\t" NUL "<old>" NUL "<new>" NUL     (rename/copy; campo 3 vazio)
//
// Binário (ou sem contagem) tem add/del = "-".
func parseNumstatEntries(data string) map[string]statEntry {
	out := map[string]statEntry{}
	parts := strings.Split(data, "\x00")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p == "" {
			continue
		}
		first := strings.IndexByte(p, '\t')
		if first < 0 {
			continue
		}
		rest := p[first+1:]
		second := strings.IndexByte(rest, '\t')
		if second < 0 {
			continue
		}
		addStr := p[:first]
		delStr := rest[:second]
		after := rest[second+1:] // path se for não-rename; vazio se rename/copy

		entry := statEntry{}
		if addStr == "-" || delStr == "-" {
			entry.add, entry.del = -1, -1
			entry.binary = true
		} else {
			entry.add, _ = strconv.Atoi(addStr)
			entry.del, _ = strconv.Atoi(delStr)
		}

		if after != "" {
			out[after] = entry
			continue
		}
		// Rename/copy: old + new path nos próximos dois tokens.
		i++
		if i >= len(parts) || parts[i] == "" {
			break
		}
		i++ // old path; não precisamos dele aqui (raw já traz)
		if i >= len(parts) {
			break
		}
		out[parts[i]] = entry
	}
	return out
}

func gitDiffList(dir string, args []string) ([]rawEntry, error) {
	cmd := gitCmd(dir, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, errs.Wrap(errs.CodeGit, "git diff --raw falhou", err)
	}
	return parseRawEntries(string(out)), nil
}

func gitDiffNumstat(dir string, args []string) (map[string]statEntry, error) {
	cmd := gitCmd(dir, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, errs.Wrap(errs.CodeGit, "git diff --numstat falhou", err)
	}
	return parseNumstatEntries(string(out)), nil
}
