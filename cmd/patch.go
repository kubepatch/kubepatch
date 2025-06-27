package cmd

import (
	"fmt"
	"time"

	"github.com/kubepatch/kubepatch/internal/unstr"

	"github.com/kubepatch/kubepatch/internal/patch"
	"github.com/spf13/cobra"
)

type PatchCmdOptions struct {
	Filenames        []string
	PatchFilePath    string
	Timeout          time.Duration
	Recursive        bool
	EnvsubstPrefixes []string
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

			// read patch-file, subst envs
			patchFile, err := patch.ReadPatchFile(opts.PatchFilePath, opts.EnvsubstPrefixes)
			if err != nil {
				return err
			}

			// preform the job
			rendered, err := patch.Run(manifests, patchFile)
			if err != nil {
				return nil
			}

			// print rendered
			fmt.Println(string(rendered))
			return nil
		},
	}
	cmd.Flags().StringSliceVarP(&opts.Filenames, "filename", "f", nil, "Manifest files, glob patterns, or directories to apply")
	cmd.Flags().StringVarP(&opts.PatchFilePath, "patchfile", "p", "", "Patch file")
	cmd.Flags().StringSliceVar(&opts.EnvsubstPrefixes, "envsubst-prefixes", nil, "List of prefixes, allowed for envsubst in a patch-file")

	_ = cmd.MarkFlagRequired("filename")  //nolint:errcheck
	_ = cmd.MarkFlagRequired("patchfile") //nolint:errcheck

	cmd.Flags().BoolVarP(&opts.Recursive, "recursive", "R", false, "Recurse into directories specified with --filename.")
	return cmd
}
