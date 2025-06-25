package patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
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
	if len(patchFile.Labels) > 0 {
		for _, doc := range manifests {
			applyCommonLabels(doc, patchFile.Labels)
		}
	}

	for i, doc := range manifests {
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

// labels

type FieldSpec struct {
	Path    string
	Group   string
	Kind    string
	Version string
	Create  bool
}

var labelFieldSpecs = []FieldSpec{
	// Base metadata.labels
	{Path: "metadata/labels", Create: true},

	// Workload templates
	{Path: "spec/template/metadata/labels", Create: true, Kind: "ReplicationController", Version: "v1"},
	{Path: "spec/template/metadata/labels", Create: true, Kind: "Deployment"},
	{Path: "spec/template/metadata/labels", Create: true, Kind: "ReplicaSet"},
	{Path: "spec/template/metadata/labels", Create: true, Kind: "DaemonSet"},
	{Path: "spec/template/metadata/labels", Create: true, Kind: "StatefulSet", Group: "apps"},
	{Path: "spec/volumeClaimTemplates[]/metadata/labels", Create: true, Kind: "StatefulSet", Group: "apps"},
	{Path: "spec/template/metadata/labels", Create: true, Kind: "Job", Group: "batch"},
	{Path: "spec/jobTemplate/metadata/labels", Create: true, Kind: "CronJob", Group: "batch"},
	{Path: "spec/jobTemplate/spec/template/metadata/labels", Create: true, Kind: "CronJob", Group: "batch"},

	// Selectors
	{Path: "spec/selector", Create: true, Kind: "Service", Version: "v1"},
	{Path: "spec/selector", Create: true, Kind: "ReplicationController", Version: "v1"},
	{Path: "spec/selector/matchLabels", Create: true, Kind: "Deployment"},
	{Path: "spec/selector/matchLabels", Create: true, Kind: "ReplicaSet"},
	{Path: "spec/selector/matchLabels", Create: true, Kind: "DaemonSet"},
	{Path: "spec/selector/matchLabels", Create: true, Kind: "StatefulSet", Group: "apps"},
	{Path: "spec/selector/matchLabels", Create: false, Kind: "Job", Group: "batch"},
	{Path: "spec/jobTemplate/spec/selector/matchLabels", Create: false, Kind: "CronJob", Group: "batch"},
	{Path: "spec/selector/matchLabels", Create: false, Kind: "PodDisruptionBudget", Group: "policy"},

	// Affinity & spread constraints
	{Path: "spec/template/spec/affinity/podAffinity/preferredDuringSchedulingIgnoredDuringExecution/podAffinityTerm/labelSelector/matchLabels", Create: false, Kind: "Deployment", Group: "apps"},
	{Path: "spec/template/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution/labelSelector/matchLabels", Create: false, Kind: "Deployment", Group: "apps"},
	{Path: "spec/template/spec/affinity/podAntiAffinity/preferredDuringSchedulingIgnoredDuringExecution/podAffinityTerm/labelSelector/matchLabels", Create: false, Kind: "Deployment", Group: "apps"},
	{Path: "spec/template/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution/labelSelector/matchLabels", Create: false, Kind: "Deployment", Group: "apps"},
	{Path: "spec/template/spec/topologySpreadConstraints/labelSelector/matchLabels", Create: false, Kind: "Deployment", Group: "apps"},

	{Path: "spec/template/spec/affinity/podAffinity/preferredDuringSchedulingIgnoredDuringExecution/podAffinityTerm/labelSelector/matchLabels", Create: false, Kind: "StatefulSet", Group: "apps"},
	{Path: "spec/template/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution/labelSelector/matchLabels", Create: false, Kind: "StatefulSet", Group: "apps"},
	{Path: "spec/template/spec/affinity/podAntiAffinity/preferredDuringSchedulingIgnoredDuringExecution/podAffinityTerm/labelSelector/matchLabels", Create: false, Kind: "StatefulSet", Group: "apps"},
	{Path: "spec/template/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution/labelSelector/matchLabels", Create: false, Kind: "StatefulSet", Group: "apps"},
	{Path: "spec/template/spec/topologySpreadConstraints/labelSelector/matchLabels", Create: false, Kind: "StatefulSet", Group: "apps"},

	// NetworkPolicy
	{Path: "spec/podSelector/matchLabels", Create: false, Kind: "NetworkPolicy", Group: "networking.k8s.io"},
	{Path: "spec/ingress/from/podSelector/matchLabels", Create: false, Kind: "NetworkPolicy", Group: "networking.k8s.io"},
	{Path: "spec/egress/to/podSelector/matchLabels", Create: false, Kind: "NetworkPolicy", Group: "networking.k8s.io"},
}

func matchGVK(obj *unstructured.Unstructured, spec FieldSpec) bool {
	if obj.GetKind() != spec.Kind {
		return false
	}
	group, version := parseAPIVersion(obj.GetAPIVersion())
	if spec.Group != "" && group != spec.Group {
		return false
	}
	if spec.Version != "" && version != spec.Version {
		return false
	}
	return true
}

func parseAPIVersion(apiVersion string) (group string, version string) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		return "", parts[0] // core group
	}
	return parts[0], parts[1]
}

func setNestedLabels(obj map[string]interface{}, path string, labels map[string]string, create bool) error {
	segments := strings.Split(path, "/")
	return setRecursive(obj, segments, labels, create)
}

func setRecursive(m map[string]interface{}, path []string, labels map[string]string, create bool) error {
	if len(path) == 0 {
		return nil
	}

	seg := path[0]

	// Handle arrays: segment like "volumeClaimTemplates[]"
	if strings.HasSuffix(seg, "[]") {
		key := strings.TrimSuffix(seg, "[]")
		raw, ok := m[key]
		if !ok {
			if !create {
				return nil
			}
			// Create empty array if allowed
			raw = []interface{}{}
			m[key] = raw
		}
		arr, ok := raw.([]interface{})
		if !ok {
			return fmt.Errorf("expected array at %q", key)
		}
		for i := range arr {
			item, ok := arr[i].(map[string]interface{})
			if !ok {
				continue
			}
			if err := setRecursive(item, path[1:], labels, create); err != nil {
				return err
			}
		}
		return nil
	}

	// Last segment — apply labels
	if len(path) == 1 {
		node, ok := m[seg].(map[string]interface{})
		if !ok {
			if !create {
				return nil
			}
			node = map[string]interface{}{}
			m[seg] = node
		}
		for k, v := range labels {
			node[k] = v
		}
		return nil
	}

	// Intermediate map key
	child, ok := m[seg].(map[string]interface{})
	if !ok {
		if !create {
			return nil
		}
		child = map[string]interface{}{}
		m[seg] = child
	}
	return setRecursive(child, path[1:], labels, create)
}

func applyCommonLabels(obj *unstructured.Unstructured, labels map[string]string) {
	for _, spec := range labelFieldSpecs {
		if !matchGVK(obj, spec) {
			// special case for ALL objects
			if spec.Path != "metadata/labels" {
				continue
			}
		}
		err := setNestedLabels(obj.Object, spec.Path, labels, spec.Create)
		if err != nil {
			log.Printf("label injection failed for path %q: %v", spec.Path, err)
		}
	}
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
