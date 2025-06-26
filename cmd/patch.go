package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kubepatch/kubepatch/internal/envs"

	"github.com/kubepatch/kubepatch/internal/unstr"

	"github.com/kubepatch/kubepatch/internal/patch"
	"sigs.k8s.io/yaml"

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
			patchFile, err := readPatchFile(opts.PatchFilePath, opts.EnvsubstPrefixes)
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

func readPatchFile(patchFilePath string, envsubstPrefixes []string) (*patch.FullPatchFile, error) {
	// read patches
	patchData, err := os.ReadFile(patchFilePath)
	if err != nil {
		return nil, err
	}

	// subst envs in a patch-file (if opts are set)
	if len(envsubstPrefixes) > 0 {
		envsubst := envs.NewEnvsubst([]string{}, envsubstPrefixes, true)
		patchFileAfterSubst, err := envsubst.SubstituteEnvs(string(patchData))
		if err != nil {
			return nil, err
		}
		patchData = []byte(patchFileAfterSubst)
	}

	// unmarshal to struct
	var patchFile patch.FullPatchFile
	if err := yaml.Unmarshal(patchData, &patchFile); err != nil {
		return nil, err
	}

	if err := checkPatchFile(&patchFile); err != nil {
		return nil, err
	}
	return &patchFile, nil
}

func checkPatchFile(patchFile *patch.FullPatchFile) error {
	for _, app := range patchFile.Patches {
		if strings.TrimSpace(app.Name) == "" {
			return fmt.Errorf("patch-file error: application name cannot be empty")
		}
		if len(app.Labels) == 0 {
			app.Labels = map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/managed-by": "kubepatch",
				"app":                          app.Name,
			}
		}
	}
	return nil
}
