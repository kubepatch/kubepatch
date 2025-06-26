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

	patchFile := &FullPatchFile{
		Patches: []*AppGroup{
			{
				Name:   "my-config",
				Labels: map[string]string{"env": "dev"},
				Resources: []*ResourcePatch{
					{
						Target: Target{
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
			},
		},
	}

	out, err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	assert.NoError(t, err)
	assert.Contains(t, string(out), "env: dev")
	assert.Contains(t, string(out), "foo: patched")
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
		Patches: []*AppGroup{
			{
				Name:   "my-config",
				Labels: map[string]string{"app": "ignored"},
				Resources: []*ResourcePatch{
					{
						Target: Target{
							Kind: "ConfigMap",
							Name: "not-matching",
						},
						Patches: []map[string]interface{}{
							{"op": "replace", "path": "/data/foo", "value": "patched"},
						},
					},
				},
			},
		},
	}

	out, err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	assert.NoError(t, err)

	assert.Contains(t, string(out), "app: ignored") // label injected
	assert.Contains(t, string(out), "foo: bar")     // value remains unpatched
	assert.NotContains(t, string(out), "patched")   // patch not applied
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
		Patches: []*AppGroup{
			{
				Name:   "my-config",
				Labels: map[string]string{"app": "ignored"},
				Resources: []*ResourcePatch{
					{
						Target: Target{
							Kind: "ConfigMap",
							Name: "my-config",
						},
						Patches: []map[string]interface{}{
							{"op": "bogus-op"}, // missing path
						},
					},
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

	patchFile := &FullPatchFile{
		Patches: []*AppGroup{
			{
				Name:   "my-config",
				Labels: map[string]string{"app": "ignored"},
				Resources: []*ResourcePatch{
					{
						Target: Target{
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

	patchFile := &FullPatchFile{
		Patches: []*AppGroup{
			{
				Name:   "my-config",
				Labels: map[string]string{"app": "ignored"},
				Resources: []*ResourcePatch{
					{
						Target: Target{
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

	// Input patch file with one app and one resource
	patchFile := &FullPatchFile{
		Patches: []*AppGroup{
			{
				Name: "my-app",
				Resources: []*ResourcePatch{
					{
						Target: Target{
							Kind: "ConfigMap",
							Name: "original-name",
						},
						Patches: []map[string]interface{}{
							{
								"op":    "replace",
								"path":  "/data/key",
								"value": "modified",
							},
						},
					},
				},
			},
		},
	}

	// Run the patching logic
	_, err := Run([]*unstructured.Unstructured{manifest}, patchFile)
	assert.NoError(t, err)

	// Assert the "replace /metadata/name" patch was appended
	p := patchFile.Patches[0].Resources[0].Patches
	assert.Len(t, p, 2)

	found := false
	for _, patch := range p {
		if patch["op"] == "replace" && patch["path"] == "/metadata/name" && patch["value"] == "my-app" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected metadata.name replacement patch to be injected")
}
