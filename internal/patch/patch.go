package patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/kubepatch/kubepatch/internal/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

type Target struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
}

type ResourcePatch struct {
	Target  Target                   `yaml:"target"`
	Patches []map[string]interface{} `yaml:"patches"`
}

type AppGroup struct {
	Name      string            `yaml:"name"`
	Labels    map[string]string `yaml:"labels"`
	Resources []*ResourcePatch  `yaml:"resources"`
}

type FullPatchFile struct {
	Patches []*AppGroup `yaml:"patches"`
}

func Run(manifests []*unstructured.Unstructured, patchFile *FullPatchFile) ([]byte, error) {
	for _, app := range patchFile.Patches {
		for _, res := range app.Resources {
			res.Patches = append(res.Patches, map[string]interface{}{
				"op":    "replace",
				"path":  "/metadata/name",
				"value": app.Name,
			})
		}
	}

	for i, doc := range manifests {
		for _, app := range patchFile.Patches {
			labels.ApplyCommonLabels(doc, app.Labels)

			for _, res := range app.Resources {
				if doc.GetKind() != res.Target.Kind || doc.GetName() != res.Target.Name {
					continue
				}

				jsonData, err := json.Marshal(doc)
				if err != nil {
					return nil, err
				}

				patchJSON, err := json.Marshal(res.Patches)
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
