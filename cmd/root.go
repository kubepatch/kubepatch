package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

func NewRootCmd(streams genericiooptions.IOStreams) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "kassert",
		Short:         "A CLI tool for asserting Kubernetes resource state",
		Long:          "Runs assertions against live Kubernetes resources using declarative YAML files.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})
	rootCmd.AddCommand(NewAssertCmd(streams))
	return rootCmd
}
