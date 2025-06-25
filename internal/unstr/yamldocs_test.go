package unstr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ReadDocs_FromStdin(t *testing.T) {
	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()

	content := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
  labels:
    app: test
`
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	_, err = w.WriteString(content)
	assert.NoError(t, err)
	_ = w.Close()
	os.Stdin = r

	objs, err := ReadDocs([]string{"-"}, false)
	assert.NoError(t, err)
	assert.Len(t, objs, 1)
	assert.Equal(t, "ConfigMap", objs[0].GetKind())
	assert.Equal(t, "cm", objs[0].GetName())
}

func Test_ReadDocs_FromSingleFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "obj.yaml")
	err := os.WriteFile(path, []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: secret
`), 0o600)
	assert.NoError(t, err)

	objs, err := ReadDocs([]string{path}, false)
	assert.NoError(t, err)
	assert.Len(t, objs, 1)
	assert.Equal(t, "Secret", objs[0].GetKind())
	assert.Equal(t, "secret", objs[0].GetName())
}

func Test_ReadDocs_FromMultipleYAMLDocs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "multi.yaml")
	err := os.WriteFile(path, []byte(`
apiVersion: v1
kind: Service
metadata:
  name: svc
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm
`), 0o600)
	assert.NoError(t, err)

	objs, err := ReadDocs([]string{path}, false)
	assert.NoError(t, err)
	assert.Len(t, objs, 2)
	assert.Equal(t, "Service", objs[0].GetKind())
	assert.Equal(t, "ConfigMap", objs[1].GetKind())
}

func Test_ReadDocs_RecursiveDirScan(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "subdir")
	err := os.Mkdir(sub, 0o755)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(sub, "a.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: a
`), 0o600)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(sub, "b.yaml"), []byte(`
apiVersion: v1
kind: Service
metadata:
  name: b
`), 0o600)
	assert.NoError(t, err)

	objs, err := ReadDocs([]string{tmp}, true)
	assert.NoError(t, err)
	assert.Len(t, objs, 2)
	kinds := []string{objs[0].GetKind(), objs[1].GetKind()}
	assert.ElementsMatch(t, kinds, []string{"ConfigMap", "Service"})
}

func Test_ReadDocs_ErrorOnBadYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.yaml")
	err := os.WriteFile(path, []byte("this: is: invalid: yaml"), 0o600)
	assert.NoError(t, err)

	objs, err := ReadDocs([]string{path}, false)
	assert.Nil(t, objs)
	assert.Error(t, err)
}

func Test_ReadDocs_ErrorOnMissingFile(t *testing.T) {
	objs, err := ReadDocs([]string{"not-exist.yaml"}, false)
	assert.Nil(t, objs)
	assert.Error(t, err)
}
