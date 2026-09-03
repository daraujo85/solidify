// Package version expõe a identidade da build do Solidify.
package version

import "runtime"

// Valores injetados via -ldflags no build de release.
var (
	Version = "0.0.0-dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Info descreve a build corrente.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// Get retorna a Info da build corrente.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// String devolve a linha humana de versão.
func (i Info) String() string {
	return "solidify " + i.Version + " (" + i.Commit + ", " + i.Date + ") " + i.GoVersion + " " + i.Platform
}
