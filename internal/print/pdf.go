// Headless PDF generator (SAI-090).
//
// Wrapper sobre browser headless: renderiza URL/HTML
// → PDF. Usa fallback Docker se nenhum browser
// estiver instalado no host.
package print

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PDFOptions entrada.
type PDFOptions struct {
	URL          string
	HTMLPath     string // alternativo a URL
	Output       string
	Width        string // ex: "210mm" (A4 width)
	Height       string // ex: "297mm" (A4 height)
	MarginTop    string
	MarginBottom string
	MarginLeft   string
	MarginRight  string
	Wait         time.Duration
	DockerImage  string // fallback: chromium headless
}

// ErrNoSource nem URL nem HTMLPath.
var ErrNoSource = errors.New("print: defina URL ou HTMLPath")

// ErrNoOutput output vazio.
var ErrNoOutput = errors.New("print: output vazio")

// GeneratePDF renderiza PDF via browser headless.
func GeneratePDF(opts PDFOptions) error {
	if opts.URL == "" && opts.HTMLPath == "" {
		return ErrNoSource
	}
	if opts.Output == "" {
		return ErrNoOutput
	}
	if opts.Width == "" {
		opts.Width = "210mm"
	}
	if opts.Height == "" {
		opts.Height = "297mm"
	}
	if opts.MarginTop == "" {
		opts.MarginTop = "10mm"
	}
	if opts.MarginBottom == "" {
		opts.MarginBottom = "10mm"
	}
	if opts.MarginLeft == "" {
		opts.MarginLeft = "10mm"
	}
	if opts.MarginRight == "" {
		opts.MarginRight = "10mm"
	}
	// tenta browser local primeiro.
	if br, err := DetectBrowser(); err == nil {
		return runLocal(br, opts)
	}
	// fallback Docker.
	if opts.DockerImage == "" {
		opts.DockerImage = "chromium-headless:latest"
	}
	return runDocker(opts)
}

func runLocal(br *BrowserCandidate, opts PDFOptions) error {
	args := append([]string{}, br.Args...)
	if opts.URL != "" {
		args = append(args, "--print-to-pdf="+opts.Output)
		args = append(args, opts.URL)
	} else {
		args = append(args, "--print-to-pdf="+opts.Output)
		args = append(args, "file://"+opts.HTMLPath)
	}
	args = append(args,
		"--no-pdf-header-footer",
		"--paper-width="+opts.Width,
		"--paper-height="+opts.Height,
		"--margin-top="+opts.MarginTop,
		"--margin-bottom="+opts.MarginBottom,
		"--margin-left="+opts.MarginLeft,
		"--margin-right="+opts.MarginRight,
	)
	if opts.Wait > 0 {
		args = append(args, fmt.Sprintf("--virtual-time-budget=%d", opts.Wait.Milliseconds()))
	}
	cmd := exec.Command(br.Path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runDocker fallback: roda browser em container.
func runDocker(opts PDFOptions) error {
	src := opts.URL
	if src == "" {
		src = "/src/" + filepath.Base(opts.HTMLPath)
	}
	args := []string{
		"run", "--rm",
		"-v", filepath.Dir(opts.HTMLPath) + ":/src:ro",
		"-v", filepath.Dir(opts.Output) + ":/out",
		opts.DockerImage,
		"--headless=new", "--disable-gpu", "--no-sandbox",
		"--print-to-pdf=/out/" + filepath.Base(opts.Output),
		"file://" + src,
	}
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// PDFSize helper.
func PDFSize(path string) (int64, error) {
	if path == "" {
		return 0, errors.New("path vazio")
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// IsPDF check magic bytes.
func IsPDF(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(string(data[:min(len(data), 4)]), "%PDF"), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Context helper.
func WithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		d = 60 * time.Second
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, d)
}
