package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeTempFile(t *testing.T, content string) string {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "patch.yaml")
	err := os.WriteFile(path, []byte(content), 0o600)
	assert.NoError(t, err)
	return path
}

func TestReadPatchFile_NoEnvsubst(t *testing.T) {
	content := `
name: mypatch
labels:
  env: dev
patches:
  - target:
      kind: Deployment
      name: myapp
    patches:
      - op: replace
        path: /spec/replicas
        value: 2
`
	path := writeTempFile(t, content)

	result, err := readPatchFile(path, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "mypatch", result.Name)
	assert.Equal(t, "dev", result.Labels["env"])
	assert.Len(t, result.Patches, 1)
}

func TestReadPatchFile_WithEnvsubst(t *testing.T) {
	os.Setenv("APP_ENV", "prod")
	defer os.Unsetenv("APP_ENV")

	content := `
name: envpatch
labels:
  env: $APP_ENV
patches: []
`
	path := writeTempFile(t, content)

	result, err := readPatchFile(path, []string{"APP_"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "envpatch", result.Name)
	assert.Equal(t, "prod", result.Labels["env"])
}

func TestReadPatchFile_InvalidYAML(t *testing.T) {
	content := `
name: invalid
labels:
  env: [unclosed
`
	path := writeTempFile(t, content)

	result, err := readPatchFile(path, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestReadPatchFile_FileNotFound(t *testing.T) {
	result, err := readPatchFile("non-existent-file.yaml", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestReadPatchFile_EnvsubstError(t *testing.T) {
	// simulate substitution error using prefix that doesn't match any defined env
	content := `
name: broken
labels:
  env: $MISSING_ENV
patches: []
`
	path := writeTempFile(t, content)

	result, err := readPatchFile(path, []string{"MISSING_"})
	assert.Error(t, err)
	assert.Nil(t, result)
}
