package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"sigs.k8s.io/yaml"
)

type PatchTarget struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
}

type PatchGroup struct {
	Target  PatchTarget              `yaml:"target"`
	When    string                   `yaml:"when"`
	Patches []map[string]interface{} `yaml:"patches"`
}

type FullPatchFile struct {
	Labels  map[string]string `yaml:"labels"`
	Patches []PatchGroup      `yaml:"patches"`
}

func main() {
	ctxArgs := flag.String("context", "", "Comma-separated context overrides (e.g. key=val,foo=bar)")
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		fmt.Println("Usage: katomik [--context key=val,...] <manifest-dir-or-file> <patches.yaml>")
		os.Exit(1)
	}

	manifestPath := args[0]
	patchFilePath := args[1]

	manifests, err := loadManifests(manifestPath)
	if err != nil {
		panic(err)
	}

	patchData, err := os.ReadFile(patchFilePath)
	if err != nil {
		panic(err)
	}

	var patchFile FullPatchFile
	if err := yaml.Unmarshal(patchData, &patchFile); err != nil {
		panic(err)
	}

	ctx := getEnvContext()
	if *ctxArgs != "" {
		overrides := strings.Split(*ctxArgs, ",")
		for _, kv := range overrides {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				ctx[parts[0]] = parts[1]
			}
		}
	}

	if len(patchFile.Labels) > 0 {
		for _, doc := range manifests {
			applyCommonLabels(doc, patchFile.Labels)
		}
	}

	for i, doc := range manifests {
		meta := getMetadata(doc)
		if meta == nil {
			continue
		}
		for _, group := range patchFile.Patches {
			if group.Target.Kind != meta.Kind || group.Target.Name != meta.Name {
				continue
			}
			if group.When != "" && !evaluateCondition(group.When, ctx) {
				continue
			}

			rawOps := []map[string]interface{}{}
			for _, patch := range group.Patches {
				if cond, ok := patch["when"].(string); ok {
					if !evaluateCondition(cond, ctx) {
						continue
					}
					delete(patch, "when")
				}
				rawOps = append(rawOps, patch)
			}

			if len(rawOps) == 0 {
				continue
			}

			jsonData, _ := json.Marshal(doc)
			patchJson, _ := json.Marshal(rawOps)

			if _, err := jsonpatch.DecodePatch(patchJson); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid patch for %s/%s: %v\n", meta.Kind, meta.Name, err)
				os.Exit(1)
			}

			patch, _ := jsonpatch.DecodePatch(patchJson)
			patchedJson, err := patch.Apply(jsonData)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to apply patch to %s/%s: %v\n", meta.Kind, meta.Name, err)
				os.Exit(1)
			}

			var updated map[string]interface{}
			_ = json.Unmarshal(patchedJson, &updated)
			manifests[i] = updated
		}
	}

	for _, doc := range manifests {
		out, _ := yaml.Marshal(doc)
		fmt.Printf("---\n%s", string(out))
	}
}

type Metadata struct {
	Kind string
	Name string
}

func getMetadata(obj map[string]interface{}) *Metadata {
	kind, _ := obj["kind"].(string)
	meta, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	name, _ := meta["name"].(string)
	return &Metadata{Kind: kind, Name: name}
}

func loadManifests(path string) ([]map[string]interface{}, error) {
	var manifests []map[string]interface{}
	files := []string{}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		err := filepath.Walk(path, func(p string, info fs.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".yaml") {
				return nil
			}
			files = append(files, p)
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		files = append(files, path)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		yamlDocs := bytes.Split(data, []byte("---"))
		for _, doc := range yamlDocs {
			doc = bytes.TrimSpace(doc)
			if len(doc) == 0 {
				continue
			}
			var obj map[string]interface{}
			if err := yaml.Unmarshal(doc, &obj); err != nil {
				return nil, err
			}
			manifests = append(manifests, obj)
		}
	}

	return manifests, nil
}

func getEnvContext() map[string]string {
	ctx := map[string]string{}
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			ctx[parts[0]] = parts[1]
		}
	}
	return ctx
}

func evaluateCondition(expr string, ctx map[string]string) bool {
	expr = strings.TrimSpace(expr)
	clauses := strings.Split(expr, "&&")
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if strings.Contains(clause, "!=") {
			parts := strings.SplitN(clause, "!=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			if ctx[key] == val {
				return false
			}
		} else if strings.Contains(clause, "==") {
			parts := strings.SplitN(clause, "==", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			if ctx[key] != val {
				return false
			}
		} else {
			return false
		}
	}
	return true
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

func matchGVK(obj map[string]interface{}, spec FieldSpec) bool {
	kind, ok := obj["kind"].(string)
	if !ok || kind != spec.Kind {
		return false
	}

	apiVersion, _ := obj["apiVersion"].(string)
	if apiVersion == "" {
		return false
	}

	group, version := parseAPIVersion(apiVersion)
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

func applyCommonLabels(obj map[string]interface{}, labels map[string]string) {
	for _, spec := range labelFieldSpecs {
		if !matchGVK(obj, spec) {
			// special case for ALL objects
			if spec.Path != "metadata/labels" {
				continue
			}
		}
		err := setNestedLabels(obj, spec.Path, labels, spec.Create)
		if err != nil {
			log.Printf("label injection failed for path %q: %v", spec.Path, err)
		}
	}
}
