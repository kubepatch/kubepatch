package patch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

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

	patchFile := FullPatchFile{
		"my-config": {
			"configmap/my-config": {
				{
					Op:    "replace",
					Path:  "/data/foo",
					Value: "patched",
				},
			},
		},
	}

	out, err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	assert.NoError(t, err)
	assert.Contains(t, string(out), "app.kubernetes.io/name: my-config") // label added
	assert.Contains(t, string(out), "foo: patched")                      // patch applied
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

	patchFile := FullPatchFile{
		"my-config": {
			"configmap/my-config": {
				{
					Op: "bogus-op",
				},
			},
		},
	}

	_, err := Run([]*unstructured.Unstructured{manifest}, patchFile)
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

	patchFile := FullPatchFile{
		"my-config": {
			"configmap/my-config": {
				{
					Op:   "remove",
					Path: "/data/missing",
				},
			},
		},
	}

	_, err := Run([]*unstructured.Unstructured{manifest}, patchFile)
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

	patchFile := FullPatchFile{
		"my-config": {
			"configmap/my-config": {
				{
					Op:    "replace",
					Path:  "/data/missing",
					Value: "nope",
				},
			},
		},
	}

	_, err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	assert.Error(t, err)
}

func TestRun_MetadataNameInjectionPreservesInput(t *testing.T) {
	manifest := mustObj(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: myconfig
`)

	originalOps := []Operation{
		{
			Op:    "add",
			Path:  "/metadata/labels/env",
			Value: "test",
		},
	}

	patchFile := FullPatchFile{
		"newname": {
			"configmap/myconfig": append([]Operation{}, originalOps...), // clone to be safe
		},
	}

	// Save deep copy of original patch input
	patchJSONBefore, err := json.Marshal(patchFile["newname"]["configmap/myconfig"])
	require.NoError(t, err)

	// Run the patch logic
	out, err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	require.NoError(t, err)

	// Assert output includes name change
	assert.Contains(t, string(out), "name: newname")

	// Ensure original patch list is NOT mutated
	patchJSONAfter, err := json.Marshal(patchFile["newname"]["configmap/myconfig"])
	require.NoError(t, err)

	assert.Equal(t, string(patchJSONBefore), string(patchJSONAfter),
		"original patch operations should remain unchanged")
}
