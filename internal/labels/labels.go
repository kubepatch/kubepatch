package labels

import (
	"fmt"
	"log"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ApplyCommonLabels(obj *unstructured.Unstructured, labels map[string]string) {
	if len(labels) == 0 {
		return
	}
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

type fieldSpec struct {
	Path    string
	Group   string
	Kind    string
	Version string
	Create  bool
}

var labelFieldSpecs = []fieldSpec{
	// Base metadata.labels
	{Path: "metadata/labels", Create: true},

	// Workload templates
	{Path: "spec/template/metadata/labels", Create: true, Kind: "ReplicationController", Version: "v1"},
	{Path: "spec/template/metadata/labels", Create: true, Kind: "Deployment"},
	{Path: "spec/template/metadata/labels", Create: true, Kind: "ReplicaSet"},
	{Path: "spec/template/metadata/labels", Create: true, Kind: "DaemonSet"},
	{Path: "spec/template/metadata/labels", Create: true, Kind: "StatefulSet", Group: "apps"},
	{Path: "spec/volumeClaimTemplates[]/metadata/labels", Create: false, Kind: "StatefulSet", Group: "apps"}, // NOTE:changes
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

func matchGVK(obj *unstructured.Unstructured, spec fieldSpec) bool {
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

func parseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		return "", parts[0] // core group
	}
	return parts[0], parts[1]
}

// v2

// setNestedLabels applies the given labels at the location defined by `path`.
// It supports path components like "containers[]/env[]/valueFrom".
func setNestedLabels(obj map[string]interface{}, path string, labels map[string]string, create bool) error {
	parts := strings.Split(path, "/")
	return applyAtPath(obj, parts, labels, create)
}

func applyAtPath(curr interface{}, parts []string, labels map[string]string, create bool) error {
	if len(parts) == 0 {
		// end of path, apply labels
		node, ok := curr.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected map at leaf, got %T", curr)
		}
		for k, v := range labels {
			node[k] = v
		}
		return nil
	}

	key := parts[0]
	isList := strings.HasSuffix(key, "[]")
	key = strings.TrimSuffix(key, "[]")

	switch node := curr.(type) {
	case map[string]interface{}:
		child, found := node[key]

		// create missing intermediate
		if !found {
			if !create {
				return nil
			}
			if isList {
				child = []interface{}{map[string]interface{}{}}
			} else {
				child = map[string]interface{}{}
			}
			node[key] = child
		}

		if isList {
			slice, ok := child.([]interface{})
			if !ok {
				return fmt.Errorf("expected slice at %q, got %T", key, child)
			}
			for _, item := range slice {
				if err := applyAtPath(item, parts[1:], labels, create); err != nil {
					return err
				}
			}
			return nil
		}

		return applyAtPath(child, parts[1:], labels, create)

	case []interface{}:
		// iterate over elements (used when applying into a slice of maps)
		for _, item := range node {
			if err := applyAtPath(item, parts, labels, create); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unexpected type at path step %q: %T", parts[0], curr)
	}
}
