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
