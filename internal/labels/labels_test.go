package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseAPIVersion(t *testing.T) {
	tests := []struct {
		input         string
		expectedGroup string
		expectedVer   string
	}{
		{"v1", "", "v1"},
		{"apps/v1", "apps", "v1"},
		{"networking.k8s.io/v1", "networking.k8s.io", "v1"},
	}

	for _, tt := range tests {
		group, version := parseAPIVersion(tt.input)
		assert.Equal(t, tt.expectedGroup, group)
		assert.Equal(t, tt.expectedVer, version)
	}
}

func TestMatchGVK(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")

	spec := fieldSpec{Group: "apps", Version: "v1", Kind: "Deployment"}
	assert.True(t, matchGVK(obj, spec))

	spec.Kind = "StatefulSet"
	assert.False(t, matchGVK(obj, spec))

	spec = fieldSpec{Group: "", Version: "v1", Kind: "Deployment"}
	assert.True(t, matchGVK(obj, spec))

	spec = fieldSpec{Group: "apps", Version: "v1", Kind: "ReplicaSet"}
	assert.False(t, matchGVK(obj, spec))
}

func TestSetNestedLabels_CreateMissing(t *testing.T) {
	obj := map[string]interface{}{}
	err := setNestedLabels(obj, "spec/template/metadata/labels", map[string]string{"foo": "bar"}, true)
	assert.NoError(t, err)

	labelsMap, found, err := unstructured.NestedStringMap(obj, "spec", "template", "metadata", "labels")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "bar", labelsMap["foo"])
}

func TestSetNestedLabels_NoCreate(t *testing.T) {
	obj := map[string]interface{}{}
	err := setNestedLabels(obj, "spec/template/metadata/labels", map[string]string{"foo": "bar"}, false)
	assert.NoError(t, err)

	found := false
	_, ok, _ := unstructured.NestedStringMap(obj, "spec", "template", "metadata", "labels")
	if ok {
		found = true
	}
	assert.False(t, found)
}

func TestSetNestedLabels_WithArray(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"volumeClaimTemplates": []interface{}{
				map[string]interface{}{
					"metadata": map[string]interface{}{},
				},
			},
		},
	}
	err := setNestedLabels(obj, "spec/volumeClaimTemplates[]/metadata/labels", map[string]string{"env": "prod"}, true)
	assert.NoError(t, err)

	arr := obj["spec"].(map[string]interface{})["volumeClaimTemplates"].([]interface{})
	item := arr[0].(map[string]interface{})["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	assert.Equal(t, "prod", item["env"])
}

func TestApplyCommonLabels_BaseCase(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test")

	ApplyCommonLabels(obj, map[string]string{"team": "devops"})

	lbls := obj.GetLabels()
	assert.Equal(t, "devops", lbls["team"])
}
