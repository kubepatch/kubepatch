package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempFile(t *testing.T, content string) string {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "patch.yaml")
	err := os.WriteFile(file, []byte(content), 0o600)
	require.NoError(t, err)
	return file
}

func TestReadPatchFile_FileNotFound(t *testing.T) {
	_, err := readPatchFile("nonexistent.yaml", nil)
	assert.Error(t, err)
}

func TestReadPatchFile_InvalidYAML(t *testing.T) {
	path := writeTempFile(t, `invalid: [unclosed`)
	_, err := readPatchFile(path, nil)
	assert.Error(t, err)
}

func TestReadPatchFile_ValidYAML(t *testing.T) {
	content := `
myapp:
  configmap/myconfig:
    - op: replace
      path: /data/foo
      value: bar
`
	path := writeTempFile(t, content)
	patchFile, err := readPatchFile(path, nil)
	require.NoError(t, err)

	ops := patchFile["myapp"]["configmap/myconfig"]
	require.Len(t, ops, 1)
	assert.Equal(t, "replace", ops[0].Op)
	assert.Equal(t, "/data/foo", ops[0].Path)
	assert.Equal(t, "bar", ops[0].Value)
}

func TestReadPatchFile_Envsubst(t *testing.T) {
	os.Setenv("FOO", "bar")
	content := `
myapp:
  configmap/myconfig:
    - op: replace
      path: /data/foo
      value: ${FOO}
`
	path := writeTempFile(t, content)
	patchFile, err := readPatchFile(path, []string{"FOO"})
	require.NoError(t, err)

	ops := patchFile["myapp"]["configmap/myconfig"]
	require.Len(t, ops, 1)
	assert.Equal(t, "bar", ops[0].Value)
}

func TestReadPatchFile_EnvsubstFails(t *testing.T) {
	content := `
myapp:
  configmap/myconfig:
    - op: replace
      path: /data/foo
      value: ${MISSING_VAR}
`
	path := writeTempFile(t, content)
	_, err := readPatchFile(path, []string{"MISSING_VAR"})
	assert.Error(t, err)
}
