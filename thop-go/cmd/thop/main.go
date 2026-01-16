package main

import (
	"fmt"
	"os"

	"github.com/scottgl9/thop/internal/cli"
)

// Build-time variables (set by ldflags)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	app := cli.NewApp(Version, GitCommit, BuildTime)
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
