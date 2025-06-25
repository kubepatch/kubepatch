package utils

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func IsClusterDefinition(object *unstructured.Unstructured) bool {
	switch {
	case IsCRD(object):
		return true
	case IsNamespace(object):
		return true
	default:
		return false
	}
}

func IsCRD(object *unstructured.Unstructured) bool {
	return strings.ToLower(object.GetKind()) == "customresourcedefinition" &&
		strings.HasPrefix(object.GetAPIVersion(), "apiextensions.k8s.io/")
}

func IsNamespace(object *unstructured.Unstructured) bool {
	return strings.ToLower(object.GetKind()) == "namespace" && object.GetAPIVersion() == "v1"
}

func IsKubernetesObject(object *unstructured.Unstructured) bool {
	if object.GetName() == "" || object.GetKind() == "" || object.GetAPIVersion() == "" {
		return false
	}
	return true
}

func IsKustomization(object *unstructured.Unstructured) bool {
	return strings.ToLower(object.GetKind()) == "kustomization" &&
		strings.HasPrefix(object.GetAPIVersion(), "kustomize.config.k8s.io/")
}

func IsSecret(object *unstructured.Unstructured) bool {
	return strings.ToLower(object.GetKind()) == "secret" && object.GetAPIVersion() == "v1"
}
