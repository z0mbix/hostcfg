package main

import (
	"github.com/z0mbix/hostcfg/internal/cli"

	// Import resources to register them
	_ "github.com/z0mbix/hostcfg/internal/resource"
)

// Build-time variables set by goreleaser
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetVersionInfo(version, commit, date)
	cli.Execute()
}
