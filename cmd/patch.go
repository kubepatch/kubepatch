package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/hashmap-kz/kubepatch/internal/unstr"

	"github.com/hashmap-kz/kubepatch/internal/patch"
	"sigs.k8s.io/yaml"

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
		RunE: func(_ *cobra.Command, _ []string) error {
			// read manifests
			manifests, err := unstr.ReadDocs(opts.Filenames, opts.Recursive)
			if err != nil {
				return err
			}

			// read patches
			patchData, err := os.ReadFile(opts.PatchFilePath)
			if err != nil {
				return err
			}
			var patchFile patch.FullPatchFile
			if err := yaml.Unmarshal(patchData, &patchFile); err != nil {
				return err
			}

			// preform the job
			rendered, err := patch.Run(manifests, &patchFile)
			if err != nil {
				return nil
			}
			fmt.Print(string(rendered))
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&opts.Filenames, "filename", "f", nil, "Manifest files, glob patterns, or directories to apply.")
	cmd.Flags().StringVarP(&opts.PatchFilePath, "patchfile", "p", "", "Patch file")

	_ = cmd.MarkFlagRequired("filename")  //nolint:errcheck
	_ = cmd.MarkFlagRequired("patchfile") //nolint:errcheck

	cmd.Flags().BoolVarP(&opts.Recursive, "recursive", "R", false, "Recurse into directories specified with --filename.")
	return cmd
}
