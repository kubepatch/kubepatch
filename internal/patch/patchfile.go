package patch

import (
	"os"

	"github.com/kubepatch/kubepatch/internal/envs"

	"sigs.k8s.io/yaml"
)

func ReadPatchFile(patchFilePath string, envsubstPrefixes []string) (FullPatchFile, error) {
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
	var patchFile FullPatchFile
	if err := yaml.Unmarshal(patchData, &patchFile); err != nil {
		return nil, err
	}
	return patchFile, nil
}
