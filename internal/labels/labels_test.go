package labels

import (
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

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

// test with manifests

const complexManifests = `
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
---
apiVersion: batch/v1
kind: Job
metadata:
  name: example-job
spec:
  template:
    metadata:
      labels:
        existing: yes
    spec:
      containers:
      - name: job
        image: busybox
        command: ["sleep", "10"]
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: example-sts
spec:
  selector:
    matchLabels:
      existing: yes
  template:
    metadata:
      labels:
        existing: yes
    spec:
      affinity:
        podAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                existing: yes
            topologyKey: zone
  volumeClaimTemplates:
  - metadata:
      name: data
---
apiVersion: v1
kind: Service
metadata:
  name: example-svc
spec:
  selector:
    existing: yes
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: example-pdb
spec:
  selector:
    matchLabels:
      existing: yes
`

func parseManifests(t *testing.T, yamlText string) []*unstructured.Unstructured {
	parts := strings.Split(yamlText, "---")
	var objs []*unstructured.Unstructured
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var obj map[string]interface{}
		err := yaml.Unmarshal([]byte(part), &obj)
		assert.NoError(t, err)
		u := &unstructured.Unstructured{Object: obj}
		objs = append(objs, u)
	}
	return objs
}

func TestApplyCommonLabels_MultiResourceTypes(t *testing.T) {
	objs := parseManifests(t, complexManifests)
	lbls := map[string]string{"injected": "true"}

	for _, obj := range objs {
		t.Run(obj.GetKind(), func(t *testing.T) {
			ApplyCommonLabels(obj, lbls)

			flat := obj.UnstructuredContent()
			switch obj.GetKind() {
			case "CustomResourceDefinition":
				// Should only inject into metadata.labels
				v, _, err := unstructured.NestedString(flat, "metadata", "labels", "injected")
				assert.NoError(t, err)
				assert.Equal(t, "true", v)

			case "Job":
				v, _, err := unstructured.NestedString(flat, "spec", "template", "metadata", "labels", "injected")
				assert.NoError(t, err)
				assert.Equal(t, "true", v)

			case "StatefulSet":
				_, found, _ := unstructured.NestedString(flat, "spec", "template", "metadata", "labels", "injected")
				assert.True(t, found)

				_, found, _ = unstructured.NestedString(flat, "spec", "selector", "matchLabels", "injected")
				assert.True(t, found)

				// volumeClaimTemplates[0].metadata.injected
				vcts, _, _ := unstructured.NestedSlice(flat, "spec", "volumeClaimTemplates")
				if len(vcts) > 0 {
					vct := vcts[0].(map[string]interface{})
					_, found := vct["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["injected"]
					assert.True(t, found)
				}

			case "Service":
				_, found, _ := unstructured.NestedString(flat, "spec", "selector", "injected")
				assert.True(t, found)

			case "PodDisruptionBudget":
				// Create=false, but matchLabels exist - label should be injected
				v, found, _ := unstructured.NestedString(flat, "spec", "selector", "matchLabels", "injected")
				assert.True(t, found)
				assert.Equal(t, "true", v)
			}
		})
	}
}

// per kind

var lbl = map[string]string{"app": "demo"}

func mustObj(y string) *unstructured.Unstructured {
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: m}
}

func Test_ReplicationController_LabelInjection(t *testing.T) {
	original := `
apiVersion: v1
kind: ReplicationController
metadata:
  name: rc
  labels: {}
spec:
  selector: {}
  template:
    metadata:
      labels: {}
    spec:
      containers:
        - name: c
          image: busybox
`
	expected := `
apiVersion: v1
kind: ReplicationController
metadata:
  name: rc
  labels:
    app: demo
spec:
  selector:
    app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      containers:
        - name: c
          image: busybox
`
	obj := mustObj(original)
	obj.SetAPIVersion("v1")
	obj.SetKind("ReplicationController")
	ApplyCommonLabels(obj, lbl)
	out, err := yaml.Marshal(obj.Object)
	assert.NoError(t, err)
	assert.YAMLEq(t, expected, string(out))
}

func Test_Deployment_LabelInjection(t *testing.T) {
	original := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dep
  labels: {}
spec:
  selector:
    matchLabels: {}
  template:
    metadata:
      labels: {}
    spec:
      affinity:
        podAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - podAffinityTerm:
                labelSelector:
                  matchLabels: {}
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels: {}
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - podAffinityTerm:
                labelSelector:
                  matchLabels: {}
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels: {}
      topologySpreadConstraints:
        - labelSelector:
            matchLabels: {}
      containers:
        - name: c
          image: busybox
`
	expected := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dep
  labels:
    app: demo
spec:
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      affinity:
        podAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: demo
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: demo
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: demo
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: demo
      topologySpreadConstraints:
        - labelSelector:
            matchLabels:
              app: demo
      containers:
        - name: c
          image: busybox
`
	obj := mustObj(original)
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	ApplyCommonLabels(obj, lbl)
	out, err := yaml.Marshal(obj.Object)
	assert.NoError(t, err)
	assert.YAMLEq(t, expected, string(out))
}

func Test_Job_LabelInjection(t *testing.T) {
	original := `
apiVersion: batch/v1
kind: Job
metadata:
  name: job
  labels: {}
spec:
  selector:
    matchLabels: {}
  template:
    metadata:
      labels: {}
    spec:
      containers:
        - name: c
          image: busybox
      restartPolicy: OnFailure
`
	expected := `
apiVersion: batch/v1
kind: Job
metadata:
  name: job
  labels:
    app: demo
spec:
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      containers:
        - name: c
          image: busybox
      restartPolicy: OnFailure
`
	obj := mustObj(original)
	obj.SetAPIVersion("batch/v1")
	obj.SetKind("Job")
	ApplyCommonLabels(obj, lbl)
	out, err := yaml.Marshal(obj.Object)
	assert.NoError(t, err)
	assert.YAMLEq(t, expected, string(out))
}

func Test_Service_LabelInjection(t *testing.T) {
	original := `
apiVersion: v1
kind: Service
metadata:
  name: svc
  labels: {}
spec:
  selector: {}
  ports:
    - port: 80
`
	expected := `
apiVersion: v1
kind: Service
metadata:
  name: svc
  labels:
    app: demo
spec:
  selector:
    app: demo
  ports:
    - port: 80
`
	obj := mustObj(original)
	obj.SetAPIVersion("v1")
	obj.SetKind("Service")
	ApplyCommonLabels(obj, lbl)
	out, err := yaml.Marshal(obj.Object)
	assert.NoError(t, err)
	assert.YAMLEq(t, expected, string(out))
}

func Test_NetworkPolicy_LabelsInjected(t *testing.T) {
	original := `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: np
  labels: {}
spec:
  podSelector:
    matchLabels: {}
  ingress:
    - from:
        - podSelector:
            matchLabels: {}
  egress:
    - to:
        - podSelector:
            matchLabels: {}
  policyTypes: [Ingress, Egress]
`

	expected := `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: np
  labels:
    app: demo
spec:
  podSelector:
    matchLabels:
      app: demo
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: demo
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: demo
  policyTypes:
    - Ingress
    - Egress
`

	obj := mustObj(original)
	obj.SetAPIVersion("networking.k8s.io/v1")
	obj.SetKind("NetworkPolicy")

	ApplyCommonLabels(obj, map[string]string{"app": "demo"})

	gotYaml, err := yaml.Marshal(obj.Object)
	assert.NoError(t, err)
	assert.YAMLEq(t, expected, string(gotYaml))
}

func Test_StatefulSet_LabelInjection(t *testing.T) {
	original := `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: sts
spec:
  template:
    spec:
      topologySpreadConstraints:
        - labelSelector: {}
      containers:
        - name: c
          image: busybox
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
`
	expected := `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: sts
  labels:
    app: demo
spec:
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      topologySpreadConstraints:
        - labelSelector: {}
      containers:
        - name: c
          image: busybox
  volumeClaimTemplates:
    - metadata:
        name: data
        labels:
          app: demo
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
`

	obj := mustObj(original)
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("StatefulSet")
	ApplyCommonLabels(obj, lbl)

	out, err := yaml.Marshal(obj.Object)
	assert.NoError(t, err)
	assert.YAMLEq(t, expected, string(out))
}
