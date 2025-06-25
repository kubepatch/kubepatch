package patch

import (
	"encoding/json"
	"fmt"
	"os"

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
	Labels  map[string]string `yaml:"labels"`
	Patches []PatchGroup      `yaml:"patches"`
}

func Run(manifests []*unstructured.Unstructured, patchFile *FullPatchFile) error {
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
				return err
			}

			patch, _ := jsonpatch.DecodePatch(patchJson)
			patchedJson, err := patch.Apply(jsonData)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to apply patch to %s/%s: %v\n", doc.GetKind(), doc.GetName(), err)
				return err
			}

			var updated unstructured.Unstructured
			_ = json.Unmarshal(patchedJson, &updated)
			manifests[i] = &updated
		}
	}

	for _, doc := range manifests {
		out, _ := yaml.Marshal(doc)
		fmt.Printf("---\n%s", string(out))
	}

	return nil
}
