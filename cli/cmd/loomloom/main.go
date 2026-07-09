package main

import (
	"fmt"
	"os"

	"github.com/Cogfoundry-ai/loomloom/cli/internal/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
