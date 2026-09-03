// Browser detection + headless PDF (SAI-089, SAI-090).
//
// Detector de browser headless disponível no host.
// Preferência: chromium → chrome → google-chrome →
// msedge → headless_shell.
package print

import (
	"errors"
	"os/exec"
	"runtime"
)

// BrowserCandidate candidato.
type BrowserCandidate struct {
	Name string
	Path string
	Args []string
}

// Common candidates por plataforma.
func CommonCandidates() []BrowserCandidate {
	switch runtime.GOOS {
	case "darwin":
		return []BrowserCandidate{
			{Name: "chrome", Path: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}},
			{Name: "chromium", Path: "/Applications/Chromium.app/Contents/MacOS/Chromium", Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}},
			{Name: "edge", Path: "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge", Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}},
		}
	case "windows":
		return []BrowserCandidate{
			{Name: "chrome", Path: `C:\Program Files\Google\Chrome\Application\chrome.exe`, Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}},
			{Name: "msedge", Path: `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}},
		}
	default: // linux.
		return []BrowserCandidate{
			{Name: "chromium", Path: "/usr/bin/chromium", Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}},
			{Name: "chromium-browser", Path: "/usr/bin/chromium-browser", Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}},
			{Name: "chrome", Path: "/usr/bin/google-chrome", Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}},
			{Name: "chrome-stable", Path: "/usr/bin/google-chrome-stable", Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}},
		}
	}
}

// ErrNoBrowser nenhum browser disponível.
var ErrNoBrowser = errors.New("print: nenhum browser headless encontrado")

// DetectBrowser acha primeiro browser disponível.
func DetectBrowser() (*BrowserCandidate, error) {
	for _, c := range CommonCandidates() {
		if pathExists(c.Path) {
			cp := c
			return &cp, nil
		}
	}
	// fallback: PATH lookup.
	for _, name := range []string{"chromium", "chrome", "google-chrome", "msedge"} {
		if p, err := exec.LookPath(name); err == nil {
			return &BrowserCandidate{Name: name, Path: p, Args: []string{"--headless=new", "--disable-gpu", "--no-sandbox"}}, nil
		}
	}
	return nil, ErrNoBrowser
}

// pathExists helper.
func pathExists(p string) bool {
	if p == "" {
		return false
	}
	// simples — fs.Stat.
	if _, err := exec.LookPath(p); err == nil {
		return true
	}
	// tenta absoluto.
	c := exec.Command("test", "-f", p)
	if err := c.Run(); err == nil {
		return true
	}
	// windows.
	c2 := exec.Command("cmd", "/c", "if exist \""+p+"\" exit 0")
	if err := c2.Run(); err == nil {
		return true
	}
	return false
}

// HasBrowser check rápido.
func HasBrowser() bool {
	_, err := DetectBrowser()
	return err == nil
}
