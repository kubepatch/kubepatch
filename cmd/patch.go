package cmd

import (
	"fmt"

	"github.com/kubepatch/kubepatch/internal/unstr"

	"github.com/kubepatch/kubepatch/internal/patch"
	"github.com/spf13/cobra"
)

type PatchCmdOptions struct {
	Filenames        []string
	PatchFilePath    string
	Recursive        bool
	EnvsubstPrefixes []string
}

func NewPatchCmd() *cobra.Command {
	opts := PatchCmdOptions{}
	cmd := &cobra.Command{
		Use:           "patch",
		SilenceErrors: true,
		SilenceUsage:  true,
		Short:         "Render Kubernetes YAML by overlaying a JSON-patch file",
		Long: `Patch reads one or more *base* manifests, applies the specified
JSON-Patch overlay, and prints the rendered manifest set to stdout.

Base manifests remain template-free; all environment-specific changes live
in the patch file.  The output can be piped straight to kubectl or stored
for GitOps diffing.`,

		Example: `
  # Render dev manifests and apply them to the cluster
  kubepatch patch -f base/ -p patches/dev.yaml | kubectl apply -f -

  # Recursively patch everything under ./k8s and diff against the cluster
  kubepatch patch -f ./k8s -R -p patches/prod.yaml | kubectl diff -f -

  # Allow ${CI_*} substitutions inside the patch file
  CI_IMAGE_TAG=1.23.4 \
  kubepatch patch \
      -f base/ \
      -p patches/ci.yaml \
      --envsubst-prefixes CI_`,

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
	cmd.Flags().BoolVarP(&opts.Recursive, "recursive", "R", false, "Recurse into directories specified with --filename.")
	cmd.Flags().StringSliceVar(&opts.EnvsubstPrefixes, "envsubst-prefixes", nil, "List of prefixes, allowed for envsubst in a patch-file")

	_ = cmd.MarkFlagRequired("filename")  //nolint:errcheck
	_ = cmd.MarkFlagRequired("patchfile") //nolint:errcheck
	return cmd
}
