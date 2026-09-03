// Command solidify é o binário único do core.
package main

import (
	"os"

	"github.com/diegoaraujo/solidify/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], app.Env{Stdout: os.Stdout, Stderr: os.Stderr}))
}
