package cmd

import (
	"time"

	"github.com/hashmap-kz/kubepatch/internal/patch"

	"github.com/spf13/cobra"
)

type PatchCmdOptions struct {
	Filenames     []string
	PatchFilePath string
	Timeout       time.Duration
	Recursive     bool
}

func NewPatchCmd() *cobra.Command {
	opts := PatchCmdOptions{}
	cmd := &cobra.Command{
		Use:           "patch",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return patch.Run(opts.Filenames, opts.PatchFilePath)
		},
	}
	cmd.Flags().StringSliceVarP(&opts.Filenames, "filename", "f", nil, "Manifest files, glob patterns, or directories to apply.")
	cmd.Flags().StringVarP(&opts.PatchFilePath, "patchfile", "p", "", "Patch file")

	//nolint:errcheck
	_ = cmd.MarkFlagRequired("filename")
	_ = cmd.MarkFlagRequired("patchfile")

	cmd.Flags().BoolVarP(&opts.Recursive, "recursive", "R", false, "Recurse into directories specified with --filename.")
	return cmd
}
