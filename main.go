package main

import (
	"fmt"
	"os"

	"github.com/hashmap-kz/kubepatch/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "exit with error: %v\n", err)
		os.Exit(1)
	}
}
