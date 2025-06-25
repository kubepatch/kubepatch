package patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/hashmap-kz/kubepatch/internal/labels"
	"github.com/hashmap-kz/kubepatch/internal/resolve"
	"github.com/hashmap-kz/kubepatch/internal/utils"

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

// ReadDocs resolves -f arguments (or stdin '-') into a slice of decoded
// Kubernetes objects. It expands directory globs, walks recursively if
// requested and supports YAML documents containing multiple resources.
func ReadDocs(filenames []string, recursive bool) ([]*unstructured.Unstructured, error) {
	var allDocs []*unstructured.Unstructured

	// 1. stdin mode: exactly one filename equal to "-"
	if len(filenames) == 1 && filenames[0] == "-" {
		d, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		docs, err := utils.ReadObjects(bytes.NewReader(d))
		if err != nil {
			return nil, err
		}
		allDocs = append(allDocs, docs...)
		return allDocs, nil
	}

	// 2. file paths & directories
	files, err := resolve.ResolveAllFiles(filenames, recursive)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		fileContent, err := resolve.ReadFileContent(file)
		if err != nil {
			return nil, err
		}
		docs, err := utils.ReadObjects(bytes.NewReader(fileContent))
		if err != nil {
			return nil, err
		}
		allDocs = append(allDocs, docs...)
	}

	return allDocs, nil
}
