package patch

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func mustObj(y string) *unstructured.Unstructured {
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: m}
}

func captureStdout(f func()) string {
	var buf bytes.Buffer
	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	_ = w.Close()
	_, _ = buf.ReadFrom(r)
	os.Stdout = stdout
	return buf.String()
}

func Test_Run_InjectLabelsAndPatch(t *testing.T) {
	manifest := mustObj(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  labels: {}
data:
  foo: bar
`)

	patchFile := &FullPatchFile{
		Labels: map[string]string{"env": "dev"},
		Patches: []PatchGroup{
			{
				Target: PatchTarget{
					Kind: "ConfigMap",
					Name: "my-config",
				},
				Patches: []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/data/foo",
						"value": "patched",
					},
				},
			},
		},
	}

	out := captureStdout(func() {
		err := Run([]*unstructured.Unstructured{manifest}, patchFile)
		assert.NoError(t, err)
	})

	assert.Contains(t, out, "env: dev")
	assert.Contains(t, out, "foo: patched")
}

func Test_Run_SkipNonMatchingTarget(t *testing.T) {
	manifest := mustObj(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: another-config
data:
  foo: bar
`)

	patchFile := &FullPatchFile{
		Labels: map[string]string{"app": "ignored"},
		Patches: []PatchGroup{
			{
				Target: PatchTarget{
					Kind: "ConfigMap",
					Name: "not-matching",
				},
				Patches: []map[string]interface{}{
					{"op": "replace", "path": "/data/foo", "value": "patched"},
				},
			},
		},
	}

	out := captureStdout(func() {
		err := Run([]*unstructured.Unstructured{manifest}, patchFile)
		assert.NoError(t, err)
	})

	assert.Contains(t, out, "app: ignored") // label injected
	assert.Contains(t, out, "foo: bar")     // value remains unpatched
	assert.NotContains(t, out, "patched")   // patch not applied
}

func Test_Run_InvalidPatchFormat(t *testing.T) {
	manifest := mustObj(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  foo: bar
`)

	patchFile := &FullPatchFile{
		Patches: []PatchGroup{
			{
				Target: PatchTarget{
					Kind: "ConfigMap",
					Name: "my-config",
				},
				Patches: []map[string]interface{}{
					{"op": "bogus-op"}, // missing path
				},
			},
		},
	}

	err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	assert.Error(t, err)
}

func Test_Run_FailedApply(t *testing.T) {
	manifest := mustObj(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  foo: bar
`)

	patchFile := &FullPatchFile{
		Patches: []PatchGroup{
			{
				Target: PatchTarget{
					Kind: "ConfigMap",
					Name: "my-config",
				},
				Patches: []map[string]interface{}{
					{
						"op":   "remove",
						"path": "/data/missing", // not present -> still valid (JSON patch allows this)
					},
				},
			},
		},
	}

	err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	assert.Error(t, err)
}

func Test_Run_ReplaceMissingField_ShouldFail(t *testing.T) {
	manifest := mustObj(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  foo: bar
`)

	patchFile := &FullPatchFile{
		Patches: []PatchGroup{
			{
				Target: PatchTarget{
					Kind: "ConfigMap",
					Name: "my-config",
				},
				Patches: []map[string]interface{}{
					{
						"op":    "replace",
						"path":  "/data/missing", // will fail: "replace" must match
						"value": "nope",
					},
				},
			},
		},
	}

	err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	assert.Error(t, err)
}
