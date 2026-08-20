package main

import (
	"fmt"
	"os"

	"github.com/lukeisontheroad/bexio-cli/internal/cmd"
)

// Set via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	if err := cmd.Execute(version); err != nil {
		fmt.Fprintln(os.Stderr, "bexio:", err)
		os.Exit(1)
	}
}
