package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/diegoaraujo/solidify/internal/config"
	"github.com/diegoaraujo/solidify/internal/errs"
)

// runInit implementa `solidify init`: prepara .solidify/ + solidify.json num
// repositório Git já existente. É idempotente: rodar duas vezes não muda o
// estado e devolve sucesso. Sobrescrever config existente exige --force.
func runInit(args []string, env Env, logger *slog.Logger) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "sobrescrever solidify.json se já existir")
	name := fs.String("name", "", "nome do projeto (default: basename do git root)")
	dir := fs.String("dir", "", "diretório alvo (default: git root)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, werr := io.WriteString(env.Stdout, "uso: solidify init [--force] [--name=NOME] [--dir=CAMINHO]\n")
			return werr
		}
		return errs.Wrap(errs.CodeUsage, "flags inválidas para init", err)
	}

	target, err := resolveInitTarget(*dir)
	if err != nil {
		return err
	}

	projectName := strings.TrimSpace(*name)
	if projectName == "" {
		projectName = filepath.Base(target)
	}
	if projectName == "." || projectName == string(filepath.Separator) || projectName == "" {
		return errs.New(errs.CodeUsage, "nome de projeto inválido; informe --name").
			WithField("project.name")
	}

	cfgPath := filepath.Join(target, config.FileName)
	artifactDir := filepath.Join(target, ".solidify")

	// solidify.json já existe? sem --force, sai com sucesso sem mexer.
	// Idempotência exigida pelo aceite da task.
	if _, statErr := os.Stat(cfgPath); statErr == nil {
		if !*force {
			logger.Info("já inicializado", "path", cfgPath)
			fmt.Fprintf(env.Stdout, "solidify já inicializado em %s\n", target)
			return nil
		}
		logger.Info("sobrescrevendo config", "path", cfgPath)
	} else if !os.IsNotExist(statErr) {
		return errs.Wrap(errs.CodeIO, "falha ao inspecionar "+cfgPath, statErr)
	}

	// .solidify/ é o artifact_dir. Criamos se faltar; se já existir, mantemos.
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return errs.Wrap(errs.CodeIO, "falha ao criar "+artifactDir, err)
	}

	cfg := config.Default()
	cfg.Project.Name = projectName
	// Aplica o artifact_dir também no path criado (consistência).
	cfg.Project.ArtifactDir = ".solidify"

	if err := cfg.Validate(); err != nil {
		return errs.Wrap(errs.CodeConfig, "config gerada é inválida", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return errs.Wrap(errs.CodeInternal, "falha ao serializar config", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		return errs.Wrap(errs.CodeIO, "falha ao escrever "+cfgPath, err)
	}

	logger.Info("inicializado", "path", target, "config", cfgPath, "artifact_dir", artifactDir)
	fmt.Fprintf(env.Stdout, "solidify inicializado em %s\n", target)
	return nil
}

// resolveInitTarget decide o diretório onde criar .solidify/ e solidify.json:
//  1. --dir, se dado (precisa estar dentro de um repo Git);
//  2. o ancestral mais próximo que contenha um .git a partir do cwd.
//
// Erros:
//   - cwd/--dir não está em nenhum repo → errs.CodeGit.
func resolveInitTarget(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", errs.Wrap(errs.CodeIO, "falha ao resolver --dir", err)
		}
		root, ok := gitRootFrom(abs)
		if !ok {
			return "", errs.New(errs.CodeGit, "--dir não está dentro de um repositório Git").
				WithField("dir")
		}
		return root, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", errs.Wrap(errs.CodeIO, "falha ao obter diretório atual", err)
	}
	root, ok := gitRootFrom(wd)
	if !ok {
		return "", errs.New(errs.CodeGit, "não foi encontrado repositório Git no diretório atual ou ancestrais").
			WithField("cwd").
			WithHint("rode `solidify init` de dentro de um repositório Git, ou use --dir")
	}
	return root, nil
}

// gitRootFrom sobe a partir de start até achar um diretório com .git
// (arquivo ou diretório: cobre worktrees, submodules, linked worktrees).
func gitRootFrom(start string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if _, statErr := os.Stat(filepath.Join(d, ".git")); statErr == nil {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}
