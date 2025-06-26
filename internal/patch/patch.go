package patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/kubepatch/kubepatch/internal/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

type Operation struct {
	Op    string      `yaml:"op" json:"op"`
	Path  string      `yaml:"path" json:"path"`
	Value interface{} `yaml:"value,omitempty" json:"value,omitempty"`
}

// FullPatchFile map[appName]map["kind/name"] -> []PatchOperation
type FullPatchFile map[string]map[string][]Operation

func Run(manifests []*unstructured.Unstructured, patchFile FullPatchFile) ([]byte, error) {
	for i, doc := range manifests {
		for appName, resources := range patchFile {
			for resourceKey, ops := range resources {
				parts := strings.SplitN(resourceKey, "/", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid resource key: %q", resourceKey)
				}
				kind, name := strings.ToLower(parts[0]), strings.ToLower(parts[1]) // normalize kind casing

				if strings.ToLower(doc.GetKind()) != kind || strings.ToLower(doc.GetName()) != name {
					continue
				}

				labels.ApplyCommonLabels(doc, map[string]string{
					"app":                    appName,
					"app.kubernetes.io/name": appName,
				})

				// Inject metadata.name patch
				opsWithName := append([]Operation{}, ops...) // clone to avoid modifying original
				opsWithName = append(opsWithName, Operation{
					Op:    "replace",
					Path:  "/metadata/name",
					Value: appName,
				})

				jsonData, err := json.Marshal(doc)
				if err != nil {
					return nil, err
				}

				patchJSON, err := json.Marshal(opsWithName)
				if err != nil {
					return nil, err
				}

				patch, err := jsonpatch.DecodePatch(patchJSON)
				if err != nil {
					return nil, fmt.Errorf("invalid patch for %s/%s: %w", kind, name, err)
				}

				patchedJSON, err := patch.Apply(jsonData)
				if err != nil {
					return nil, fmt.Errorf("failed to apply patch to %s/%s: %w", kind, name, err)
				}

				var updated unstructured.Unstructured
				if err := json.Unmarshal(patchedJSON, &updated); err != nil {
					return nil, err
				}
				manifests[i] = &updated
			}
		}
	}

	var buf bytes.Buffer
	for _, doc := range manifests {
		out, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		buf.WriteString("---\n")
		buf.Write(out)
	}
	return buf.Bytes(), nil
}
