// Command semstat reports facts about semantic version strings.
package main

import (
	"os"

	"github.com/loft-sh/semstat/internal/cli"
)

// Set by goreleaser at build time.
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version, os.Stdout, os.Stderr))
}
