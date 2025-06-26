package patch

import (
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
	assert.Contains(t, string(out), "app: my-config") // label added
	assert.Contains(t, string(out), "foo: patched")   // patch applied
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

func Test_Run_AppendsNameReplacementPatch(t *testing.T) {
	// Initial manifest
	manifest := mustObj(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: original-name
data:
  key: value
`)

	patchFile := FullPatchFile{
		"my-app": {
			"configmap/original-name": {
				{
					Op:    "replace",
					Path:  "/data/key",
					Value: "modified",
				},
			},
		},
	}

	// Run the patching logic
	_, err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	assert.NoError(t, err)
}
