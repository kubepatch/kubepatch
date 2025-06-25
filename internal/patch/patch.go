package patch

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/hashmap-kz/kubepatch/internal/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

type PatchTarget struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
}

type PatchGroup struct {
	Target  PatchTarget              `yaml:"target"`
	Patches []map[string]interface{} `yaml:"patches"`
}

type FullPatchFile struct {
	Name    string            `yaml:"name"`
	Labels  map[string]string `yaml:"labels"`
	Patches []*PatchGroup     `yaml:"patches"`
}

func Run(manifests []*unstructured.Unstructured, patchFile *FullPatchFile) (string, error) {
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

			jsonData, _ := json.Marshal(doc)
			patchJson, _ := json.Marshal(group.Patches)

			if _, err := jsonpatch.DecodePatch(patchJson); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid patch for %s/%s: %v\n", doc.GetKind(), doc.GetName(), err)
				return "", err
			}

			patch, _ := jsonpatch.DecodePatch(patchJson)
			patchedJson, err := patch.Apply(jsonData)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to apply patch to %s/%s: %v\n", doc.GetKind(), doc.GetName(), err)
				return "", err
			}

			var updated unstructured.Unstructured
			_ = json.Unmarshal(patchedJson, &updated)
			manifests[i] = &updated
		}
	}

	sb := strings.Builder{}
	for _, doc := range manifests {
		out, _ := yaml.Marshal(doc)
		sb.WriteString(fmt.Sprintf("---\n%s", string(out)))
	}
	return sb.String(), nil
}
