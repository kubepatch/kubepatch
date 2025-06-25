package patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/hashmap-kz/kubepatch/internal/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

type Target struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
}

type Group struct {
	Target  Target                   `yaml:"target"`
	Patches []map[string]interface{} `yaml:"patches"`
}

type FullPatchFile struct {
	Name    string            `yaml:"name"`
	Labels  map[string]string `yaml:"labels"`
	Patches []*Group          `yaml:"patches"`
}

func Run(manifests []*unstructured.Unstructured, patchFile *FullPatchFile) ([]byte, error) {
	// metadata.name
	for _, group := range patchFile.Patches {
		group.Patches = append(group.Patches, map[string]interface{}{
			"op":    "replace",
			"path":  "/metadata/name",
			"value": patchFile.Name,
		})
	}

	for i, doc := range manifests {
		labels.ApplyCommonLabels(doc, patchFile.Labels)

		for _, group := range patchFile.Patches {
			if group.Target.Kind != doc.GetKind() || group.Target.Name != doc.GetName() {
				continue
			}

			jsonData, err := json.Marshal(doc)
			if err != nil {
				return nil, err
			}

			patchJSON, err := json.Marshal(group.Patches)
			if err != nil {
				return nil, err
			}

			patch, err := jsonpatch.DecodePatch(patchJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid patch for %s/%s: %v\n", doc.GetKind(), doc.GetName(), err)
				return nil, err
			}

			patchedJSON, err := patch.Apply(jsonData)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to apply patch to %s/%s: %v\n", doc.GetKind(), doc.GetName(), err)
				return nil, err
			}

			var updated unstructured.Unstructured
			err = json.Unmarshal(patchedJSON, &updated)
			if err != nil {
				return nil, err
			}
			manifests[i] = &updated
		}
	}

	buf := bytes.Buffer{}
	for _, doc := range manifests {
		out, err := yaml.Marshal(doc)
		if err != nil {
			return nil, err
		}
		buf.WriteString(fmt.Sprintf("---\n%s", string(out)))
	}
	return buf.Bytes(), nil
}
