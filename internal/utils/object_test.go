package utils

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestReadManifests(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantObjs int
	}{
		{
			name: "single valid manifest",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
`,
			wantErr:  false,
			wantObjs: 1,
		},
		{
			name: "multiple manifests with separator",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-2
`,
			wantErr:  false,
			wantObjs: 2,
		},
		{
			name: "empty document ignored",
			input: `
---
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-final
`,
			wantErr:  false,
			wantObjs: 1,
		},
		{
			name: "invalid yaml document",
			input: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: broken
  namespace: default
  - oops
`,
			wantErr: true,
		},
		{
			name:     "completely empty input",
			input:    ``,
			wantErr:  false,
			wantObjs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs, err := ReadObjects(bytes.NewReader([]byte(tt.input)))

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantObjs, len(objs))
				for _, obj := range objs {
					assert.IsType(t, &unstructured.Unstructured{}, obj)
					assert.NotEmpty(t, obj.GetKind())
				}
			}
		})
	}
}

func TestReadObjects_DropsInvalid(t *testing.T) {
	testCases := []struct {
		name      string
		resources string
		expected  int
	}{
		{
			name: "valid resources",
			resources: `
---
apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: default
immutable: true
stringData:
  key: "private-key"
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-provider-aws2
spec:
  package: crossplane/provider-aws:v0.23.0
  controllerConfigRef:
    name: provider-aws
`,
			expected: 2,
		},
		{
			name: "some invalid resources",
			resources: `
---
piVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: crossplane-provider-aws1
spec:
  package: crossplane/provider-aws:v0.23.0
  controllerConfigRef:
    name: provider-aws
---
apiVersion: v1
kind: Secret
metadata:
  name: test
  namespace: default
immutable: true
stringData:
  key: "private-key"
`,
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			objects, err := ReadObjects(strings.NewReader(tc.resources))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(objects) != tc.expected {
				t.Errorf("unexpected number of objects in %v", objects)
			}
		})
	}
}
