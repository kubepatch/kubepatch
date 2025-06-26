package unstr

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/kubepatch/kubepatch/internal/resolve"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
		docs, err := ReadObjects(bytes.NewReader(d))
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
		docs, err := ReadObjects(bytes.NewReader(fileContent))
		if err != nil {
			return nil, err
		}
		allDocs = append(allDocs, docs...)
	}

	return allDocs, nil
}

func DeepCloneManifests(in []*unstructured.Unstructured) []*unstructured.Unstructured {
	if in == nil {
		return nil
	}
	out := make([]*unstructured.Unstructured, len(in))
	for i, obj := range in {
		if obj != nil {
			out[i] = obj.DeepCopy()
		}
	}
	return out
}
